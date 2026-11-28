import sqlite3
import tarfile
from io import StringIO
from pathlib import Path

import pytest
from django.core.management import call_command, get_commands
from django.core.management.base import CommandError
from django.urls import reverse

from tests.factories import ActivityFactory, seed_dev_ingest_token
from trove.management.commands.backup import _reject_memory
from trove.models import Activity
from trove.runtime.core import reset_core
from trove.services.backup import unpack_tarball, write_tarball
from trove.services.owner import ensure_owner

pytestmark = pytest.mark.django_db(databases=["default", "data"])


def _write_sqlite(path: Path, script: str):
    conn = sqlite3.connect(path)
    try:
        conn.executescript(script)
        conn.commit()
    finally:
        conn.close()


def test_backup_writes_sqlite_then_blobs_without_storage(tmp_path):
    config = tmp_path / "config.sqlite3"
    data = tmp_path / "data.sqlite3"
    blobs = tmp_path / "blobs"
    _write_sqlite(
        config,
        "create table trove_configentry (domain text);"
        "insert into trove_configentry values ('ingest');",
    )
    _write_sqlite(data, "create table trove_item (id text);")
    sha = "ab" * 32
    blob_path = blobs / sha[:2] / sha
    blob_path.parent.mkdir(parents=True)
    blob_path.write_bytes(b"keep-me")
    dest = tmp_path / "backup.tar.gz"

    write_tarball(dest, config_path=config, data_path=data, blob_dir=blobs)

    assert dest.is_file()
    with tarfile.open(dest, "r:gz") as tar:
        names = tar.getnames()
        assert names[:2] == ["config.sqlite3", "data.sqlite3"]
        blob_member = f"blobs/{sha[:2]}/{sha}"
        assert blob_member in names
        assert names.index(blob_member) > names.index("data.sqlite3")
        assert not any(
            name == ".storage" or name.startswith(".storage/") for name in names
        )
        config_file = tar.extractfile("config.sqlite3")
        data_file = tar.extractfile("data.sqlite3")
        blob_file = tar.extractfile(blob_member)
        assert config_file is not None
        assert data_file is not None
        assert blob_file is not None
        config_bytes = config_file.read()
        data_bytes = data_file.read()
        assert blob_file.read() == b"keep-me"

    extracted = tmp_path / "extracted"
    extracted.mkdir()
    (extracted / "config.sqlite3").write_bytes(config_bytes)
    (extracted / "data.sqlite3").write_bytes(data_bytes)
    copied = sqlite3.connect(extracted / "config.sqlite3")
    try:
        domains = [
            row[0]
            for row in copied.execute("select domain from trove_configentry").fetchall()
        ]
    finally:
        copied.close()
    assert "ingest" in domains
    copied_data = sqlite3.connect(extracted / "data.sqlite3")
    try:
        tables = {
            row[0]
            for row in copied_data.execute(
                "select name from sqlite_master where type='table'"
            ).fetchall()
        }
    finally:
        copied_data.close()
    assert "trove_item" in tables


def test_backup_keeps_generated_secret_for_restore(tmp_path):
    config = tmp_path / "config.sqlite3"
    data = tmp_path / "data.sqlite3"
    secret = tmp_path / ".secret-key"
    _write_sqlite(config, "create table config (id integer);")
    _write_sqlite(data, "create table data (id integer);")
    secret.write_text("test-secret")
    secret.chmod(0o600)
    archive = tmp_path / "backup.tar.gz"
    write_tarball(
        archive,
        config_path=config,
        data_path=data,
        blob_dir=tmp_path / "blobs",
        secret_key_path=secret,
    )

    assert archive.stat().st_mode & 0o777 == 0o600
    with tarfile.open(archive, "r:gz") as tar:
        member = tar.getmember(".secret-key")
        restored = tar.extractfile(member)
        assert restored is not None
        assert restored.read() == b"test-secret"
        assert member.mode == 0o600
    unpack_tarball(archive, tmp_path / "restore")
    restored_secret = tmp_path / "restore" / ".secret-key"
    assert restored_secret.read_text() == "test-secret"
    assert restored_secret.stat().st_mode & 0o777 == 0o600


def test_backup_rejects_in_memory_database():
    with pytest.raises(CommandError, match="in-memory"):
        _reject_memory(Path("file:memorydb_default?mode=memory&cache=shared"))


def test_backup_command_is_registered():
    assert "backup" in get_commands()


def test_processes_renders_one_trove_program():
    stdout = StringIO()
    call_command("processes", stdout=stdout)
    text = stdout.getvalue()
    assert "[program:trove]" in text
    assert "manage.py trove" in text
    assert text.count("[program:") == 1
    assert "mcp" not in text.lower()
    assert "telegram" not in text.lower()
    assert "ox_worker" not in text


def test_status_lists_plugins_and_failures():
    reset_core()
    ActivityFactory(
        kind="derive",
        agent="palette@1",
        outcome=Activity.Outcome.FAILED,
        error="palette exploded",
    )
    stdout = StringIO()
    call_command("status", stdout=stdout)
    text = stdout.getvalue()
    assert "plugins" in text
    assert "ingest" in text
    assert "loaded" in text
    assert "failures" in text
    assert "palette@1" in text
    assert "palette exploded" in text


def test_admin_shows_activities_failures_plugins_tokens(client):
    reset_core()
    owner = ensure_owner()
    client.force_login(owner)
    ActivityFactory(
        kind="derive",
        agent="palette@1",
        outcome=Activity.Outcome.FAILED,
        error="palette exploded",
    )
    seed_dev_ingest_token()

    activities = client.get(reverse("admin:trove_activity_changelist"))
    assert activities.status_code == 200
    content = activities.content.decode()
    assert "palette exploded" in content
    assert "failed" in content

    plugins = client.get(reverse("admin:trove_configentry_changelist"))
    assert plugins.status_code == 200
    plugin_page = plugins.content.decode()
    assert "ingest" in plugin_page
    assert "loaded" in plugin_page

    tokens = client.get(reverse("admin:oauth2_provider_accesstoken_changelist"))
    assert tokens.status_code == 200
    assert tokens.content
