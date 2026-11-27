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
    n = int(log.read_text()) if log.exists() else 0
    log.write_text(str(n + 1))
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
        f"'WORKER_CONCURRENCY': 1}}\n"
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


def _seed_slow_job(dest: Path, plugin_root: str) -> str:
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
    subprocess.run(
        [sys.executable, str(seed)],
        check=True,
        cwd=str(settings.BASE_DIR),
        env=env,
        capture_output=True,
        text=True,
    )
    return marker.read_text()


def _file_db_script(dest: Path, plugin_root: str, body: str) -> str:
    env = _ox_worker_env(dest / "_settings", plugin_root, dest)
    script = dest / "_settings" / "query.py"
    script.write_text(
        "import django\n"
        "django.setup()\n"
        "from trove.core import reset_core\n"
        "reset_core()\n" + body
    )
    result = subprocess.run(
        [sys.executable, str(script)],
        check=True,
        cwd=str(settings.BASE_DIR),
        env=env,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip().splitlines()[-1]


def _ok_count(dest: Path, plugin_root: str, item_id: str) -> int:
    return int(
        _file_db_script(
            dest,
            plugin_root,
            "from trove.models import Activity\n"
            f"print(Activity.objects.filter(agent='slow@1', "
            f"subject_id={item_id!r}, outcome='ok').count())\n",
        )
    )


def _file_db_reconcile(dest: Path, plugin_root: str) -> int:
    return int(
        _file_db_script(
            dest,
            plugin_root,
            "from trove.services.reconcile import reconcile\nprint(reconcile())\n",
        )
    )


def _ox_count(dest: Path, where: str = "") -> int:
    conn = sqlite3.connect(dest / "config.sqlite3")
    try:
        sql = "SELECT COUNT(*) FROM django_ox_oxtask"
        if where:
            sql += f" WHERE {where}"
        return conn.execute(sql).fetchone()[0]
    finally:
        conn.close()


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
    item_id = _seed_slow_job(dest, root)
    assert _ox_count(dest) == 1

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
                "1",
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

    assert _ox_count(dest, "status = 'RUNNING'") == 1
    assert _file_db_reconcile(dest, root) == 1
    time.sleep(0.6)
    restart = env.copy()
    restart.pop("TROVE_HOLD", None)
    restarted = _manage_py(
        restart,
        "ox_worker",
        "--processes",
        "1",
        "--concurrency",
        "1",
        "--lock-timeout",
        "0.4",
        "--batch",
    )
    assert _ox_count(dest, "status != 'SUCCESSFUL'") == 0
    assert run_log.read_text() == "1", restarted.stderr or restarted.stdout
    assert _ok_count(dest, root, item_id) == 1
