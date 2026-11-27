import io
import json
import tempfile
from pathlib import Path

from django.conf import settings
from django.core.exceptions import ImproperlyConfigured
from django.db import IntegrityError, connections, transaction
from django.test import TestCase, override_settings
from django_ox.models import OxTask
from django_ox.testing import run_tasks
from PIL import Image
from pydantic import ValidationError

from tests.factories import ConfigEntryFactory, ItemFactory
from trove.core import Processor, get_core, reset_core
from trove.models import Activity, AppendOnlyError, Claim, Item, Link
from trove.services.reconcile import reconcile
from trove.services.recorder import record
from trove.tasks import Emit, Result, TransientError, enqueue_derive, execute_derive

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}
TMP = tempfile.mkdtemp()


def png(colour="#336699", second=(230, 120, 40)):
    img = Image.new("RGB", (100, 100), colour)
    img.paste(second, (0, 0, 40, 100))
    buf = io.BytesIO()
    img.save(buf, "PNG")
    buf.name = "x.png"
    buf.seek(0)
    return buf


@override_settings(TROVE={**settings.TROVE, "BLOB_DIR": Path(TMP) / "blobs"})
class CoreTests(TestCase):
    databases = {"default", "data"}

    def capture(self, **extra):
        r = self.client.post(
            "/ingest/", {"file": png(), "title": "nice palette", **extra}, **TOKEN
        )
        self.assertEqual(r.status_code, 201, r.content)
        return Item.objects.get(id=r.json()["id"])

    def test_two_databases(self):
        def tables(db):
            return connections[db].introspection.table_names()

        self.assertIn("trove_item", tables("data"))
        self.assertNotIn("trove_item", tables("default"))
        self.assertIn("trove_configentry", tables("default"))
        self.assertNotIn("auth_user", tables("data"))
        self.assertIn("django_ox_oxtask", tables("data"))
        self.assertNotIn("django_ox_oxtask", tables("default"))

    def test_append_only(self):
        item = self.capture()
        with self.assertRaises(AppendOnlyError):
            item.save()
        with self.assertRaises(AppendOnlyError):
            Claim.objects.filter(item=item).update(value="x")
        for sql in ("UPDATE trove_claim SET value='\"x\"'", "DELETE FROM trove_item"):
            with self.assertRaises(IntegrityError), transaction.atomic(using="data"):
                connections["data"].cursor().execute(sql)

    def test_ingest(self):
        item = self.capture(notes="for the hallway")
        values = item.current_values()
        self.assertEqual((values["width"], values["height"]), (100, 100))
        self.assertEqual(values["notes"], "for the hallway")
        act = item.activity
        self.assertEqual(
            (act.kind, act.agent, act.outcome), ("ingest", "shortcut:iphone", "ok")
        )
        self.assertTrue((Path(TMP) / "blobs" / values["$blob"][:2]).exists())
        self.assertEqual(self.client.post("/ingest/", {"url": "x"}).status_code, 401)

    def test_reconcile_derive_and_idempotence(self):
        item = self.capture()
        self.assertEqual(OxTask.objects.count(), 2)
        run_tasks()
        colours = sorted(
            Item.objects.filter(type="colour").values_list("id", flat=True)
        )
        self.assertEqual(len(colours), 2)
        self.assertEqual(
            item.current_values()["summary"], "A image titled 'nice palette'"
        )
        self.assertEqual(reconcile(), 0)

        get_core().processors["palette"].version = 2
        self.assertEqual(reconcile(), 1)
        run_tasks()
        again = sorted(Item.objects.filter(type="colour").values_list("id", flat=True))
        self.assertEqual(colours, again)
        get_core().processors["palette"].version = 1

        wall = self.client.get("/wall/api/items").json()["items"][0]
        self.assertEqual(sorted(wall["colours"]), ["#336699", "#e67828"])

    def test_rank_then_recency(self):
        item = ItemFactory()
        with record("derive", "agent@1", subject=item) as r:
            r.claim(item, "base", "E14")
        with record("edit", "user") as r:
            r.claim(item, "base", "E27", rank=Claim.Rank.PREFERRED)
        with record("derive", "agent@2", subject=item) as r:
            r.claim(item, "base", "E14")
        self.assertEqual(item.current_values()["base"], "E27")
        with record("edit", "user") as r:
            r.claim(item, "base", "E27", rank=Claim.Rank.DEPRECATED)
        self.assertEqual(item.current_values()["base"], "E14")

    def test_links_assert_and_retract(self):
        a, b = ItemFactory(), ItemFactory(type="capture.file")
        with record("edit", "user") as r:
            link = r.link(a, b, "fits")
        self.assertEqual(Link.objects.live().filter(rel="fits").count(), 1)
        with record("edit", "user") as r:
            r.retract(link)
        self.assertEqual(Link.objects.live().filter(rel="fits").count(), 0)

    def test_validation_rolls_back_whole_activity(self):
        before = Activity.objects.count()
        with self.assertRaises(ValidationError):
            with record("edit", "user") as r:
                c = r.item("colour")
                r.claim(c, "hex", 123)
                r.claim(c, "share", "lots")
        self.assertEqual(Item.objects.filter(type="colour").count(), 0)
        self.assertEqual(Activity.objects.count(), before)

    def test_transient_then_success_and_permanent_failure(self):
        item, calls = self.capture(), []

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
        self.assertEqual(len(calls), 3)
        self.assertTrue(item.current_values()["flaky"])

        def broken(i, ctx):
            raise ValueError("bad response")

        execute_derive(Processor("broken", 1, "capture.*", [], broken), item.id)
        failed = Activity.objects.get(agent="broken@1")
        self.assertEqual(failed.outcome, "failed")
        self.assertIn("bad response", failed.error)


class PluginLoadingTests(TestCase):
    databases = {"default", "data"}

    def plugin_dir(self, plugins: dict) -> str:
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

    def load(self, root):
        reset_core()
        with override_settings(TROVE={**settings.TROVE, "PLUGIN_DIRS": [root]}):
            try:
                return get_core()
            finally:
                reset_core()

    def test_broken_builtin_disabled_and_dependents_follow(self):
        root = self.plugin_dir(
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
        core = self.load(root)
        self.assertEqual(core.plugins["good"].status, "loaded")
        self.assertEqual(core.plugins["bad"].status, "failed")
        self.assertEqual(core.plugins["needs_bad"].status, "disabled")

    def test_broken_core_plugin_stops_startup(self):
        root = self.plugin_dir(
            {"ingest2": ("core", "raise ImportError('no pillow')", [])}
        )
        with self.assertRaises(ImproperlyConfigured):
            self.load(root)

    def test_disabled_by_config_entry(self):
        root = self.plugin_dir({"opt": ("builtin", "def setup(core, entry): pass", [])})
        ConfigEntryFactory(domain="opt", enabled=False)
        self.assertEqual(self.load(root).plugins["opt"].status, "disabled")

    def test_processor_loop_refused(self):
        code = (
            "from pydantic import BaseModel\nfrom trove.core import Processor\n"
            "class T(BaseModel): pass\n"
            "def setup(core, entry):\n"
            "    core.add_type('x', T); core.add_type('y', T)\n"
            "    core.add_processor(Processor('a', 1, 'x', ['y'], lambda i, c: None))\n"
            "    core.add_processor(Processor('b', 1, 'y', ['x'], lambda i, c: None))\n"
        )
        root = self.plugin_dir({"loopy": ("core", code, [])})
        with self.assertRaises(ImproperlyConfigured):
            self.load(root)


class InvalidOutputTests(TestCase):
    databases = {"default", "data"}

    def test_invalid_processor_output_is_a_recorded_failure(self):
        with record("ingest", "test") as r:
            item = r.item("capture.link")
        bad = Processor(
            "bad_ai",
            1,
            "capture.*",
            ["colour"],
            lambda i, c: Result(emits=[Emit("colour", "k", {"hex": 1})]),
        )
        execute_derive(bad, item.id)
        act = Activity.objects.get(agent="bad_ai@1")
        self.assertEqual(act.outcome, "failed")
        self.assertIn("validation", act.error.lower())
        self.assertEqual(Item.objects.filter(type="colour").count(), 0)
