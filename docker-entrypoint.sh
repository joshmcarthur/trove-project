#!/bin/sh
set -eu

if [ -z "${DJANGO_SECRET_KEY:-}" ]; then
    TROVE_SECRET_KEY_FILE="${TROVE_DATA_DIR:-/data}/.secret-key"
    export TROVE_SECRET_KEY_FILE
    DJANGO_SECRET_KEY="$(python - <<'PY'
import os
import secrets
import tempfile
from pathlib import Path

path = Path(os.environ["TROVE_SECRET_KEY_FILE"])
if not path.exists():
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix=".secret-key-")
    try:
        with os.fdopen(fd, "w") as file:
            file.write(secrets.token_urlsafe(64))
            file.flush()
            os.fsync(file.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            pass
    finally:
        os.unlink(temporary)
secret = path.read_text().strip()
if not secret:
    raise SystemExit("The persisted Django secret is empty")
print(secret)
PY
)"
    export DJANGO_SECRET_KEY
else
    unset TROVE_SECRET_KEY_FILE
fi

if [ "$#" -eq 0 ]; then
    python manage.py migrate --noinput
    python manage.py migrate --database=data --noinput
    exec python manage.py trove
fi

exec "$@"
