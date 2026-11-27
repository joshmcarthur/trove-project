from __future__ import annotations

import hashlib
from pathlib import Path

from django.conf import settings


def blob_path(sha: str) -> Path:
    return Path(settings.TROVE["BLOB_DIR"]) / sha[:2] / sha


def write_blob(data: bytes) -> str:
    sha = hashlib.sha256(data).hexdigest()
    path = blob_path(sha)
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists():
        path.write_bytes(data)
    return sha
