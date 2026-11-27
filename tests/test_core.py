import io
import json
import tempfile
from datetime import timedelta
from pathlib import Path
from types import SimpleNamespace

import pytest
from django.conf import settings
from django.core.exceptions import ImproperlyConfigured
from django.core.files.uploadedfile import SimpleUploadedFile
from django.db import IntegrityError, connections, transaction
from django.test import override_settings
from django.utils import timezone
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
from trove.services.reap import reap_abandoned
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
    assert "trove_configentry" in tables("default")
    assert "auth_user" not in tables("data")
    assert "django_ox_oxtask" in tables("data")
    assert "django_ox_oxtask" not in tables("default")
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


def test_killed_running_task_is_reaped_and_not_duplicated(capture):
    item = capture()
    assert OxTask.objects.count() == 2
    assert reconcile() == 0
    assert OxTask.objects.count() == 2

    task = next(row for row in OxTask.objects.all() if row.args[0] == "palette")
    task.status = OxTask.Status.RUNNING
    task.locked_by = "dead-worker"
    task.locked_at = timezone.now()
    task.lease_expires_at = timezone.now() + timedelta(hours=1)
    task.save()

    assert reconcile() == 0
    assert OxTask.objects.count() == 2
    assert OxTask.objects.get(pk=task.pk).status == OxTask.Status.RUNNING

    assert reap_abandoned() >= 1
    task.refresh_from_db()
    assert task.status == OxTask.Status.READY
    assert reconcile() == 0
    assert OxTask.objects.count() == 2

    run_tasks()
    assert Item.objects.filter(type="colour").count() == 2
    assert Activity.objects.filter(agent="agent@1", subject=item, outcome="ok").exists()


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
