import io
import json
import os
import sqlite3
import subprocess
import sys
import tempfile
import time
from datetime import timedelta
from pathlib import Path
from types import SimpleNamespace

import pytest
from django.conf import settings
from django.core.exceptions import ImproperlyConfigured
from django.core.files.uploadedfile import SimpleUploadedFile
from django.db import IntegrityError, connections, transaction
from django.test import override_settings
from django_ox.models import OxTask
from django_ox.testing import run_tasks
from PIL import Image
from pydantic import ValidationError

from tests.factories import (
    ConfigEntryFactory,
    ItemFactory,
    seed_dev_ingest_token,
    seed_dev_wall_token,
)
from trove.core import Processor, get_core, reset_core
from trove.models import Activity, AppendOnlyError, Claim, Item, Link
from trove.services.reconcile import reconcile
from trove.services.recorder import record
from trove.tasks import Emit, Result, TransientError, enqueue_derive
from trove.tasks.enqueue import derive_backoff
from trove.tasks.execute import execute_derive

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture(autouse=True)
def ingest_token():
    seed_dev_ingest_token()


def png(colour="#336699", second=(230, 120, 40)):
    img = Image.new("RGB", (100, 100), colour)
    img.paste(second, (0, 0, 40, 100))
    buf = io.BytesIO()
    img.save(buf, "PNG")
    buf.name = "x.png"
    buf.seek(0)
    return buf


HEIC_FIXTURE = Path(__file__).parent / "fixtures" / "tiny.heic"


def heic(content_type="image/heic"):
    return SimpleUploadedFile("lamp.heic", HEIC_FIXTURE.read_bytes(), content_type)


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


@pytest.fixture
def capture(client, blob_dir):
    def _capture(**extra):
        r = client.post(
            "/ingest/", {"file": png(), "title": "nice palette", **extra}, **TOKEN
        )
        assert r.status_code == 201, r.content
        return Item.objects.get(id=r.json()["id"])

    return _capture


def plugin_dir(plugins: dict) -> str:
    root = Path(tempfile.mkdtemp())
    for domain, (kind, code, deps) in plugins.items():
        (root / domain).mkdir()
        (root / domain / "manifest.json").write_text(
            json.dumps(
                {
                    "domain": domain,
                    "name": domain,
                    "version": "1",
                    "kind": kind,
                    "dependencies": deps,
                }
            )
        )
        (root / domain / "__init__.py").write_text(code)
    return str(root)


def load(root):
    reset_core()
    with override_settings(TROVE={**settings.TROVE, "PLUGIN_DIRS": [root]}):
        try:
            return get_core()
        finally:
            reset_core()


def test_two_databases():
    def tables(db):
        return connections[db].introspection.table_names()

    assert "trove_item" in tables("data")
    assert "trove_item" not in tables("default")
    assert "trove_activity" in tables("data")
    assert "trove_activity" not in tables("default")
    assert "trove_configentry" in tables("default")
    assert "auth_user" not in tables("data")
    assert "django_ox_oxtask" in tables("default")
    assert "django_ox_oxtask" not in tables("data")
    assert "oauth2_provider_accesstoken" in tables("default")
    assert "oauth2_provider_accesstoken" not in tables("data")
    assert "mfa_authenticator" in tables("default")
    assert "mfa_authenticator" not in tables("data")


def test_append_only(capture):
    item = capture()
    with pytest.raises(AppendOnlyError):
        item.save()
    with pytest.raises(AppendOnlyError):
        Claim.objects.filter(item=item).update(value="x")
    for sql in ("UPDATE trove_claim SET value='\"x\"'", "DELETE FROM trove_item"):
        with pytest.raises(IntegrityError), transaction.atomic(using="data"):
            connections["data"].cursor().execute(sql)


def test_ingest(capture, client, blob_dir):
    item = capture(notes="for the hallway")
    values = item.current_values()
    assert (values["width"], values["height"]) == (100, 100)
    assert values["notes"] == "for the hallway"
    act = item.activity
    assert (act.kind, act.agent, act.outcome) == ("ingest", "shortcut:iphone", "ok")
    assert (blob_dir / values["$blob"][:2]).exists()
    assert client.post("/ingest/", {"url": "x"}).status_code == 401


def test_ingest_capture_metadata_is_preferred(capture):
    item = capture(notes="for the hallway", url="https://example.com")
    ranks = {
        claim.path: claim.rank
        for claim in Claim.objects.filter(item=item, path__in=("title", "notes", "url"))
    }
    assert ranks == {
        "title": Claim.Rank.PREFERRED,
        "notes": Claim.Rank.PREFERRED,
        "url": Claim.Rank.PREFERRED,
    }


def test_ingest_heic(client, blob_dir):
    response = client.post("/ingest/", {"file": heic()}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.image"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert (values["width"], values["height"]) == (100, 100)
    assert (blob_dir / values["$blob"][:2]).exists()


def test_palette_reads_heic(client, blob_dir):
    response = client.post("/ingest/", {"file": heic()}, **TOKEN)
    assert response.status_code == 201, response.content
    run_tasks()
    assert Item.objects.filter(type="colour").exists()


def test_ingest_heic_octet_stream(client, blob_dir):
    response = client.post(
        "/ingest/", {"file": heic("application/octet-stream")}, **TOKEN
    )
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.image"
    values = Item.objects.get(id=payload["id"]).current_values()
    assert (values["width"], values["height"]) == (100, 100)


def test_reconcile_derive_and_idempotence(capture, client):
    item = capture()
    assert OxTask.objects.count() == 2
    run_tasks()
    colours = sorted(Item.objects.filter(type="colour").values_list("id", flat=True))
    assert len(colours) == 2
    agent = Activity.objects.get(agent="agent@1", subject=item, outcome="ok")
    assert agent.params["prompt_hash"]
    assert agent.params["model"]
    assert reconcile() == 0

    get_core().processors["palette"].version = 2
    assert reconcile() == 1
    run_tasks()
    again = sorted(Item.objects.filter(type="colour").values_list("id", flat=True))
    assert colours == again
    get_core().processors["palette"].version = 1

    seed_dev_wall_token()
    wall = client.get(
        "/wall/api/items", HTTP_AUTHORIZATION="Bearer dev-wall-token"
    ).json()["items"][0]
    assert sorted(wall["colours"]) == ["#336699", "#e67828"]
    assert wall["values"]["title"] == "nice palette"
    kinds = {row["kind"] for row in wall["provenance"]}
    assert {"ingest", "derive"} <= kinds
    assert all("agent" in row and "when" in row for row in wall["provenance"])


def test_rank_then_recency():
    item = ItemFactory()
    with record("derive", "agent@1", subject=item) as r:
        r.claim(item, "base", "E14")
    with record("edit", "user") as r:
        r.claim(item, "base", "E27", rank=Claim.Rank.PREFERRED)
    with record("derive", "agent@2", subject=item) as r:
        r.claim(item, "base", "E14")
    assert item.current_values()["base"] == "E27"
    with record("edit", "user") as r:
        r.claim(item, "base", "E27", rank=Claim.Rank.DEPRECATED)
    assert item.current_values()["base"] == "E14"


def test_links_assert_and_retract():
    a, b = ItemFactory(), ItemFactory(type="capture.file")
    with record("edit", "user") as r:
        link = r.link(a, b, "fits")
    assert Link.objects.live().filter(rel="fits").count() == 1
    with record("edit", "user") as r:
        r.retract(link)
    assert Link.objects.live().filter(rel="fits").count() == 0


def test_validation_rolls_back_whole_activity():
    before = Activity.objects.count()
    with pytest.raises(ValidationError):
        with record("edit", "user") as r:
            c = r.item("colour")
            r.claim(c, "hex", 123)
            r.claim(c, "share", "lots")
    assert Item.objects.filter(type="colour").count() == 0
    assert Activity.objects.count() == before


def test_transient_then_success_and_permanent_failure(capture):
    item, calls = capture(), []

    def flaky(i, ctx):
        calls.append(1)
        if len(calls) < 3:
            raise TransientError("429", retry_after=0)
        return Result(claims={"flaky": True})

    core = get_core()
    core.add_processor(Processor("flaky", 1, "capture.*", [], flaky))
    try:
        enqueue_derive("flaky", item.id)
        run_tasks()
    finally:
        core.processors.pop("flaky", None)
    assert len(calls) == 3
    assert item.current_values()["flaky"]

    def broken(i, ctx):
        raise ValueError("bad response")

    execute_derive(Processor("broken", 1, "capture.*", [], broken), item.id)
    failed = Activity.objects.get(agent="broken@1")
    assert failed.outcome == "failed"
    assert "bad response" in failed.error


def test_transient_exhaustion_records_failed_activity():
    item = ItemFactory()

    def always_busy(i, ctx):
        raise TransientError("429", retry_after=0)

    core = get_core()
    core.add_processor(Processor("quota", 1, "capture.*", [], always_busy))
    try:
        enqueue_derive("quota", item.id)
        run_tasks()
    finally:
        core.processors.pop("quota", None)
    failed = Activity.objects.get(agent="quota@1")
    assert failed.outcome == "failed"
    assert "429" in failed.error


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
    queued = sqlite3.connect(dest / "config.sqlite3").execute(
        "SELECT COUNT(*) FROM django_ox_oxtask"
    ).fetchone()[0]
    assert queued == 2, f"seed wrote {queued} OxTask rows"

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
    assert run_log.read_text() == "1", restarted.stderr or restarted.stdout
    ok = sqlite3.connect(dest / "data.sqlite3").execute(
        "SELECT COUNT(*) FROM trove_activity "
        "WHERE agent=? AND subject_id=? AND outcome=?",
        ("slow@1", item_id, "ok"),
    ).fetchone()[0]
    assert ok == 1
    assert _file_db_reconcile(dest, root) == 0


def test_derive_backoff_returns_timedelta_for_retry_after():
    delay = derive_backoff(
        TransientError("429", retry_after=1.5), SimpleNamespace(attempts=1)
    )
    assert delay == timedelta(seconds=1.5)


def test_broken_builtin_disabled_and_dependents_follow():
    root = plugin_dir(
        {
            "good": ("builtin", "def setup(core, entry): pass", []),
            "bad": (
                "builtin",
                "def setup(core, entry): raise RuntimeError('boom')",
                [],
            ),
            "needs_bad": ("builtin", "def setup(core, entry): pass", ["bad"]),
        }
    )
    core = load(root)
    assert core.plugins["good"].status == "loaded"
    assert core.plugins["bad"].status == "failed"
    assert core.plugins["needs_bad"].status == "disabled"


def test_broken_core_plugin_stops_startup():
    root = plugin_dir({"ingest2": ("core", "raise ImportError('no pillow')", [])})
    with pytest.raises(ImproperlyConfigured):
        load(root)


def test_disabled_by_config_entry():
    root = plugin_dir({"opt": ("builtin", "def setup(core, entry): pass", [])})
    ConfigEntryFactory(domain="opt", enabled=False)
    assert load(root).plugins["opt"].status == "disabled"


def test_processor_loop_refused():
    code = (
        "from pydantic import BaseModel\nfrom trove.core import Processor\n"
        "class T(BaseModel): pass\n"
        "def setup(core, entry):\n"
        "    core.add_type('x', T); core.add_type('y', T)\n"
        "    core.add_processor(Processor('a', 1, 'x', ['y'], lambda i, c: None))\n"
        "    core.add_processor(Processor('b', 1, 'y', ['x'], lambda i, c: None))\n"
    )
    root = plugin_dir({"loopy": ("core", code, [])})
    with pytest.raises(ImproperlyConfigured):
        load(root)


def test_explicit_self_loop_is_refused():
    code = (
        "from pydantic import BaseModel\nfrom trove.core import Processor\n"
        "class T(BaseModel): pass\n"
        "def setup(core, entry):\n"
        "    core.add_type('x', T)\n"
        "    core.add_processor(Processor('loop', 1, 'x', ['x'], lambda i, c: None))\n"
    )
    root = plugin_dir({"selfie": ("core", code, [])})
    with pytest.raises(ImproperlyConfigured):
        load(root)


def test_invalid_processor_output_is_a_recorded_failure():
    item = ItemFactory(type="capture.link")
    bad = Processor(
        "bad_ai",
        1,
        "capture.*",
        ["colour"],
        lambda i, c: Result(emits=[Emit("colour", "k", {"hex": 1})]),
    )
    execute_derive(bad, item.id)
    act = Activity.objects.get(agent="bad_ai@1")
    assert act.outcome == "failed"
    assert "validation" in act.error.lower()
    assert Item.objects.filter(type="colour").count() == 0
