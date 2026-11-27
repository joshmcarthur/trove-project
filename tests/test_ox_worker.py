import os
import sqlite3
import subprocess
import sys
import time
from pathlib import Path

from django.conf import settings

from tests.test_core import plugin_dir

SLOW_PLUGIN = """
import os
import time
from pathlib import Path

from trove.core import Processor
from trove.tasks import Result


def run(item, ctx):
    hold = os.environ.get("TROVE_HOLD")
    if hold:
        Path(hold).write_text("1")
        time.sleep(60)
    log = Path(os.environ["TROVE_RUN_LOG"])
    with log.open("a") as fh:
        fh.write("1\\n")
    return Result(claims={"slow": True})


def setup(core, entry):
    core.add_processor(Processor("slow", 1, "capture.slow", [], run))
"""


def _ox_worker_env(settings_dir: Path, plugin_root: str, dest: Path) -> dict:
    settings_dir.mkdir(exist_ok=True)
    default_name = str(dest / "config.sqlite3")
    data_name = str(dest / "data.sqlite3")
    (settings_dir / "kill_settings.py").write_text(
        "from config.settings import *\n"
        f"DATABASES['default']['NAME'] = {default_name!r}\n"
        f"DATABASES['data']['NAME'] = {data_name!r}\n"
        f"TROVE = {{**TROVE, 'PLUGIN_DIRS': [{plugin_root!r}], "
        f"'WORKER_CONCURRENCY': 2}}\n"
        "TASKS = {**TASKS, 'default': {**TASKS['default'], "
        "'OPTIONS': {**TASKS['default']['OPTIONS'], 'LOCK_TIMEOUT': 0.4}}}\n"
    )
    env = os.environ.copy()
    env["DJANGO_SETTINGS_MODULE"] = "kill_settings"
    env["PYTHONUNBUFFERED"] = "1"
    env["PYTHONPATH"] = os.pathsep.join(
        [str(settings_dir), str(settings.BASE_DIR), env.get("PYTHONPATH", "")]
    )
    return env


def _manage_py(env: dict, *args: str, check=True) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(Path(settings.BASE_DIR) / "manage.py"), *args],
        check=check,
        cwd=str(settings.BASE_DIR),
        env=env,
        capture_output=True,
        text=True,
    )


def _migrate_file_dbs(dest: Path, plugin_root: str):
    env = _ox_worker_env(dest / "_settings", plugin_root, dest)
    _manage_py(env, "migrate", "--verbosity", "0")
    _manage_py(env, "migrate", "--database", "data", "--verbosity", "0")


def _seed_slow_jobs(dest: Path, plugin_root: str) -> str:
    env = _ox_worker_env(dest / "_settings", plugin_root, dest)
    marker = dest / "item_id"
    seed = dest / "_settings" / "seed.py"
    seed.write_text(
        "import django\n"
        "django.setup()\n"
        "from tests.factories import ItemFactory\n"
        "from trove.core import reset_core\n"
        "from trove.tasks import enqueue_derive\n"
        "reset_core()\n"
        "item = ItemFactory(type='capture.slow')\n"
        "enqueue_derive('slow', item.id)\n"
        f"open({str(marker)!r}, 'w').write(item.id)\n"
    )
    result = subprocess.run(
        [sys.executable, str(seed)],
        cwd=str(settings.BASE_DIR),
        env=env,
        capture_output=True,
        text=True,
    )
    if result.returncode:
        raise AssertionError(result.stderr or result.stdout)
    return marker.read_text()


def _file_db_reconcile(dest: Path, plugin_root: str) -> int:
    env = _ox_worker_env(dest / "_settings", plugin_root, dest)
    script = dest / "_settings" / "reconcile.py"
    script.write_text(
        "import django\n"
        "django.setup()\n"
        "from trove.core import reset_core\n"
        "from trove.services.reconcile import reconcile\n"
        "reset_core()\n"
        "print(reconcile())\n"
    )
    result = subprocess.run(
        [sys.executable, str(script)],
        check=True,
        cwd=str(settings.BASE_DIR),
        env=env,
        capture_output=True,
        text=True,
    )
    return int(result.stdout.strip().splitlines()[-1])


def _wait_for_hold(proc: subprocess.Popen, hold: Path, log: Path):
    deadline = time.monotonic() + 10
    while not hold.exists():
        if time.monotonic() > deadline:
            proc.kill()
            proc.wait(timeout=5)
            raise AssertionError(
                "worker never claimed the slow task\n" + log.read_text()
            )
        if proc.poll() is not None:
            raise AssertionError(
                f"worker exited early: {proc.returncode}\n" + log.read_text()
            )
        time.sleep(0.05)


def test_killed_running_task_is_reaped_and_not_duplicated(tmp_path):
    dest = tmp_path / "instance"
    dest.mkdir()
    root = plugin_dir({"slow": ("builtin", SLOW_PLUGIN, [])})
    hold = tmp_path / "hold"
    run_log = tmp_path / "runs"
    worker_log = dest / "worker.log"
    _migrate_file_dbs(dest, root)
    item_id = _seed_slow_jobs(dest, root)
    queued = (
        sqlite3.connect(dest / "config.sqlite3")
        .execute("SELECT COUNT(*) FROM django_ox_oxtask")
        .fetchone()[0]
    )
    assert queued == 1, f"seed wrote {queued} OxTask rows"

    env = _ox_worker_env(dest / "_settings", root, dest)
    env["TROVE_HOLD"] = str(hold)
    env["TROVE_RUN_LOG"] = str(run_log)
    with worker_log.open("w") as log:
        proc = subprocess.Popen(
            [
                sys.executable,
                str(Path(settings.BASE_DIR) / "manage.py"),
                "ox_worker",
                "--processes",
                "1",
                "--concurrency",
                "2",
                "--lock-timeout",
                "0.4",
            ],
            env=env,
            cwd=str(settings.BASE_DIR),
            stdout=log,
            stderr=subprocess.STDOUT,
        )
    try:
        _wait_for_hold(proc, hold, worker_log)
        proc.kill()
        proc.wait(timeout=5)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    _file_db_reconcile(dest, root)
    time.sleep(0.6)
    restart = env.copy()
    restart.pop("TROVE_HOLD", None)
    restarted = _manage_py(
        restart,
        "ox_worker",
        "--processes",
        "1",
        "--concurrency",
        "2",
        "--lock-timeout",
        "0.4",
        "--batch",
    )
    assert run_log.read_text().splitlines() == ["1"], (
        restarted.stderr or restarted.stdout
    )
    ok = (
        sqlite3.connect(dest / "data.sqlite3")
        .execute(
            "SELECT COUNT(*) FROM trove_activity "
            "WHERE agent=? AND subject_id=? AND outcome=?",
            ("slow@1", item_id, "ok"),
        )
        .fetchone()[0]
    )
    assert ok == 1
    leftover = (
        sqlite3.connect(dest / "config.sqlite3")
        .execute("SELECT COUNT(*) FROM django_ox_oxtask WHERE status != 'SUCCESSFUL'")
        .fetchone()[0]
    )
    assert leftover == 0
