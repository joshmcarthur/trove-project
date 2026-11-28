import hashlib
import json
import os
import subprocess
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

ENTRYPOINT = Path(__file__).resolve().parents[1] / "docker-entrypoint.sh"


def _start(data_dir, **extra_env):
    env = os.environ.copy()
    env.pop("DJANGO_SECRET_KEY", None)
    env.pop("TROVE_SECRET_KEY_FILE", None)
    env.update(TROVE_DATA_DIR=str(data_dir), **extra_env)
    return subprocess.run(
        [
            str(ENTRYPOINT),
            "python",
            "-c",
            "import hashlib, os; "
            "print(hashlib.sha256(os.environ['DJANGO_SECRET_KEY'].encode()).hexdigest())",
        ],
        env=env,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def test_container_generates_one_persistent_secret_for_concurrent_starts(tmp_path):
    with ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(_start, [tmp_path, tmp_path]))

    secret_file = tmp_path / ".secret-key"
    secret = secret_file.read_bytes()
    assert results == [hashlib.sha256(secret).hexdigest()] * 2
    assert secret_file.stat().st_mode & 0o777 == 0o600
    assert _start(tmp_path) == results[0]


def test_container_respects_explicit_secret_without_creating_file(tmp_path):
    result = _start(tmp_path, DJANGO_SECRET_KEY="explicit-test-secret")

    assert result == hashlib.sha256(b"explicit-test-secret").hexdigest()
    assert not (tmp_path / ".secret-key").exists()


def test_management_process_loads_generated_secret_from_data_volume(tmp_path):
    expected = _start(tmp_path)
    env = os.environ.copy()
    env.pop("DJANGO_SECRET_KEY", None)
    env.pop("TROVE_SECRET_KEY_FILE", None)
    env["TROVE_DATA_DIR"] = str(tmp_path)
    process = subprocess.run(
        [
            "python",
            "-c",
            "import hashlib; from config import settings; "
            "print(hashlib.sha256(settings.SECRET_KEY.encode()).hexdigest()); "
            "print(settings.SECRET_KEY_FILE)",
        ],
        env=env,
        capture_output=True,
        text=True,
        check=True,
    )

    assert process.stdout.splitlines() == [expected, str(tmp_path / ".secret-key")]


def test_container_settings_use_data_volume_and_allowed_hosts(tmp_path):
    env = os.environ.copy()
    env.update(
        TROVE_DATA_DIR=str(tmp_path),
        DJANGO_DEBUG="false",
        DJANGO_ALLOWED_HOSTS="trove.example.com, other.example.com",
        DJANGO_SECRET_KEY="test-secret",
    )
    process = subprocess.run(
        [
            "python",
            "-c",
            "import json; from config import settings; "
            "print(json.dumps([settings.DEBUG, settings.ALLOWED_HOSTS, "
            "str(settings.DATABASES['default']['NAME']), "
            "str(settings.DATABASES['data']['NAME']), "
            "str(settings.TROVE['BLOB_DIR'])]))",
        ],
        env=env,
        capture_output=True,
        text=True,
        check=True,
    )

    debug, hosts, config_db, data_db, blobs = json.loads(process.stdout)
    assert not debug
    assert "trove.example.com" in hosts
    assert "other.example.com" in hosts
    assert config_db == str(tmp_path / "config.sqlite3")
    assert data_db == str(tmp_path / "data.sqlite3")
    assert blobs == str(tmp_path / "blobs")


def test_debug_disabled_rejects_missing_secret(tmp_path):
    env = os.environ.copy()
    env.pop("DJANGO_SECRET_KEY", None)
    env.pop("TROVE_SECRET_KEY_FILE", None)
    env.update(TROVE_DATA_DIR=str(tmp_path), DJANGO_DEBUG="false")
    process = subprocess.run(
        ["python", "-c", "from config import settings"],
        env=env,
        capture_output=True,
        text=True,
    )

    assert process.returncode != 0
    assert "persisted Django secret is required" in process.stderr


def test_container_rejects_empty_persisted_secret(tmp_path):
    (tmp_path / ".secret-key").touch()

    try:
        _start(tmp_path)
    except subprocess.CalledProcessError as error:
        assert "persisted Django secret is empty" in error.stderr
    else:
        raise AssertionError("Startup accepted an empty persisted secret")
