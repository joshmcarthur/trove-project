from __future__ import annotations

import os
import sqlite3
import tarfile
import tempfile
from pathlib import Path


def sqlite_backup(src: Path, dest: Path):
    source = _connect(src)
    dest_conn = sqlite3.connect(str(dest))
    try:
        source.backup(dest_conn)
    finally:
        dest_conn.close()
        source.close()


def write_tarball(
    output: Path,
    *,
    config_path: Path,
    data_path: Path,
    blob_dir: Path,
    secret_key_path: Path | None = None,
):
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as raw_tmp:
        tmp = Path(raw_tmp)
        config_copy = tmp / "config.sqlite3"
        data_copy = tmp / "data.sqlite3"
        sqlite_backup(config_path, config_copy)
        sqlite_backup(data_path, data_copy)
        with output.open("wb") as file:
            os.fchmod(file.fileno(), 0o600)
            with tarfile.open(fileobj=file, mode="w:gz") as tar:
                tar.add(config_copy, arcname="config.sqlite3")
                tar.add(data_copy, arcname="data.sqlite3")
                if secret_key_path is not None:
                    tar.add(secret_key_path, arcname=".secret-key")
                _add_blobs(tar, blob_dir)


def unpack_tarball(archive: Path, dest: Path):
    dest.mkdir(parents=True, exist_ok=True)
    with tarfile.open(archive, "r:gz") as tar:
        tar.extractall(dest, filter="data")


def _connect(path: Path) -> sqlite3.Connection:
    text = str(path)
    if text.startswith("file:"):
        return sqlite3.connect(text, uri=True)
    return sqlite3.connect(text)


def _add_blobs(tar: tarfile.TarFile, blob_dir: Path):
    if not blob_dir.exists():
        return
    for path in sorted(p for p in blob_dir.rglob("*") if p.is_file()):
        tar.add(path, arcname=str(Path("blobs") / path.relative_to(blob_dir)))
