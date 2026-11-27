import io
import os
import sqlite3
import subprocess
import sys
import tarfile
from contextlib import contextmanager
from pathlib import Path

import pytest
from django.conf import settings
from django.db import connections
from django.test.utils import override_settings

from plugins.wall.queries import wall_items
from trove.models import Activity, Claim
from trove.services.backup import unpack_tarball, write_tarball
from trove.services.blobs import blob_path
from trove.services.recorder import record


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "live-blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def _write_sqlite(path: Path, script: str):
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path)
    try:
        conn.executescript(script)
        conn.commit()
    finally:
        conn.close()


def _snapshot_db(alias: str, dest: Path):
    conn = connections[alias]
    conn.ensure_connection()
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.unlink(missing_ok=True)
    target = sqlite3.connect(str(dest))
    try:
        target.executescript("\n".join(conn.connection.iterdump()))
    finally:
        target.close()


@contextmanager
def _against_restored(dest: Path):
    databases = {
        alias: {**settings.DATABASES[alias], "NAME": dest / name}
        for alias, name in (("default", "config.sqlite3"), ("data", "data.sqlite3"))
    }
    with override_settings(
        DATABASES=databases,
        TROVE={**settings.TROVE, "BLOB_DIR": dest / "blobs"},
    ):
        connections.close_all()
        try:
            yield
        finally:
            connections.close_all()


def _migrate_restored(dest: Path):
    repo = Path(settings.BASE_DIR)
    settings_dir = dest / "_settings"
    settings_dir.mkdir()
    (settings_dir / "restore_settings.py").write_text(
        "from config.settings import *\n"
        f"DATABASES['default']['NAME'] = {str(dest / 'config.sqlite3')!r}\n"
        f"DATABASES['data']['NAME'] = {str(dest / 'data.sqlite3')!r}\n"
        f"TROVE = {{**TROVE, 'BLOB_DIR': {str(dest / 'blobs')!r}}}\n"
    )
    env = os.environ.copy()
    env["DJANGO_SETTINGS_MODULE"] = "restore_settings"
    env["PYTHONPATH"] = os.pathsep.join(
        [str(settings_dir), str(repo), env.get("PYTHONPATH", "")]
    )
    for args in (
        ["migrate", "--verbosity", "0"],
        ["migrate", "--database", "data", "--verbosity", "0"],
    ):
        subprocess.run(
            [sys.executable, str(repo / "manage.py"), *args],
            check=True,
            cwd=str(repo),
            env=env,
            capture_output=True,
        )


def test_unpack_restores_sqlite_and_blobs(tmp_path):
    src = tmp_path / "src"
    config = src / "config.sqlite3"
    data = src / "data.sqlite3"
    blobs = src / "blobs"
    _write_sqlite(
        config,
        "create table trove_configentry (domain text);"
        "insert into trove_configentry values ('ingest');",
    )
    _write_sqlite(
        data,
        "create table trove_item (id text); insert into trove_item values ('i1');",
    )
    sha = "cd" * 32
    blob_path = blobs / sha[:2] / sha
    blob_path.parent.mkdir(parents=True)
    blob_path.write_bytes(b"restored-bytes")
    tarball = tmp_path / "backup.tar.gz"
    write_tarball(tarball, config_path=config, data_path=data, blob_dir=blobs)

    dest = tmp_path / "clean"
    unpack_tarball(tarball, dest)

    assert (dest / "config.sqlite3").is_file()
    assert (dest / "data.sqlite3").is_file()
    assert (dest / "blobs" / sha[:2] / sha).read_bytes() == b"restored-bytes"
    copied = sqlite3.connect(dest / "data.sqlite3")
    try:
        rows = copied.execute("select id from trove_item").fetchall()
    finally:
        copied.close()
    assert rows == [("i1",)]


def test_unpack_stays_inside_dest(tmp_path):
    archive = tmp_path / "evil.tar.gz"
    with tarfile.open(archive, "w:gz") as tar:
        payload = b"nope"
        info = tarfile.TarInfo(name="../outside.txt")
        info.size = len(payload)
        tar.addfile(info, io.BytesIO(payload))
    dest = tmp_path / "clean"
    dest.mkdir()
    with pytest.raises(tarfile.FilterError):
        unpack_tarball(archive, dest)
    assert not (tmp_path / "outside.txt").exists()


@pytest.mark.django_db(databases=["default", "data"])
@pytest.mark.filterwarnings("ignore:Overriding setting DATABASES:UserWarning")
def test_restore_keeps_ingest_and_derive_after_migrate(blob_dir, tmp_path):
    with record("ingest", "test") as rec:
        item = rec.item("capture.image")
        rec.claim(item, "$blob", rec.blob(b"hello-image", "image/png"))
        rec.claim(item, "title", "lamp", rank=Claim.Rank.PREFERRED)
    with record("derive", "palette@1", subject=item) as rec:
        colour = rec.item("colour")
        rec.claim(colour, "hex", "#336699")
        rec.link(colour, item, "derived_from")

    assert Activity.objects.filter(kind="ingest", outcome="ok").exists()
    assert Activity.objects.filter(kind="derive", outcome="ok").exists()
    sha = item.current_values()["$blob"]
    live_bytes = (blob_dir / sha[:2] / sha).read_bytes()

    snapshot = tmp_path / "snapshot"
    _snapshot_db("default", snapshot / "config.sqlite3")
    _snapshot_db("data", snapshot / "data.sqlite3")
    tarball = tmp_path / "backup.tar.gz"
    write_tarball(
        tarball,
        config_path=snapshot / "config.sqlite3",
        data_path=snapshot / "data.sqlite3",
        blob_dir=blob_dir,
    )

    dest = tmp_path / "restored"
    unpack_tarball(tarball, dest)
    _migrate_restored(dest)

    with _against_restored(dest):
        assert Path(settings.DATABASES["data"]["NAME"]) == dest / "data.sqlite3"
        assert Path(settings.TROVE["BLOB_DIR"]) == dest / "blobs"
        row = next(entry for entry in wall_items()["items"] if entry["id"] == item.id)
        assert row["type"] == "capture.image"
        assert row["values"]["title"] == "lamp"
        assert row["values"]["$blob"] == sha
        assert "#336699" in row["colours"]
        kinds = {entry["kind"] for entry in row["provenance"]}
        assert {"ingest", "derive"} <= kinds
        restored_blob = blob_path(sha)
        assert restored_blob == dest / "blobs" / sha[:2] / sha
        assert restored_blob.read_bytes() == live_bytes
