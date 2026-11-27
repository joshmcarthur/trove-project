import sys
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest

from tests.factories import ActivityFactory, ItemFactory
from trove.core import get_core, reset_core
from trove.models import Activity, Blob, Claim, Item, Link
from trove.services.fetch_error import FetchError
from trove.services.recorder import record
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])

PAGE = "https://example.com/lamp"
FIXTURE = Path(__file__).parent / "fixtures" / "lamp.mhtml"
SHELL = Path(__file__).parent / "fixtures" / "hallway-lamp.html"
MHTML = FIXTURE.read_bytes()
SHELL_HTML = SHELL.read_text()
_CHROMIUM_INSTALLED = None


def chromium_installed():
    global _CHROMIUM_INSTALLED
    if _CHROMIUM_INSTALLED is not None:
        return _CHROMIUM_INSTALLED
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        _CHROMIUM_INSTALLED = False
        return False
    try:
        with sync_playwright() as playwright:
            browser = playwright.chromium.launch(headless=True)
            browser.close()
    except Exception:
        _CHROMIUM_INSTALLED = False
        return False
    _CHROMIUM_INSTALLED = True
    return True


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def make_link(url):
    with record("ingest", "test") as rec:
        item = rec.item("capture.link")
        rec.claim(item, "url", url, rank=Claim.Rank.PREFERRED)
    return item


def archive_mod():
    return sys.modules["trove_plugins.archive.run"]


def chromium_mod():
    return sys.modules["trove_plugins.archive.chromium"]


def patch_capture(monkeypatch, fake):
    monkeypatch.setattr(archive_mod(), "capture_mhtml", fake)


def patch_hop(monkeypatch, status=200, headers=None, body=b"<html></html>"):
    response_headers = dict(headers or {})
    response_body = body

    def hop(url, *, method, headers, body, timeout):
        return status, response_headers, response_body

    monkeypatch.setattr(chromium_mod(), "fetch_hop", hop)


def returns_mhtml(body=MHTML, timeout_seen=None):
    def capture(url, *, timeout):
        if timeout_seen is not None:
            timeout_seen.append(timeout)
        return body

    return capture


def timed_out(url, *, timeout):
    raise chromium_mod().CaptureError("timed out")


def capture_failed(url, *, timeout):
    raise FetchError("HTTP 404")


def archive(core, item):
    execute_derive(core.processors["archive"], item.id)
    item.refresh_from_db()
    return item


def derived_files(link):
    ids = (
        Link.objects.live().filter(target=link, rel="derived_from").values("source_id")
    )
    return list(Item.objects.filter(id__in=ids, type="capture.file"))


class FakeFrame:
    def __init__(self, main=True):
        self.parent_frame = None if main else object()


class FakeRequest:
    def __init__(self, url, *, navigation=False, main=True):
        self.url = url
        self.method = "GET"
        self.headers = {}
        self.post_data = None
        self.post_data_buffer = None
        self._navigation = navigation
        self.frame = FakeFrame(main)

    def is_navigation_request(self):
        return self._navigation


class FakeRoute:
    def __init__(self, url, *, navigation=False, main=True):
        self.request = FakeRequest(url, navigation=navigation, main=main)
        self.actions = []
        self.fulfilled = None

    def continue_(self):
        self.actions.append("continue")

    def abort(self):
        self.actions.append("abort")

    def fulfill(self, **kwargs):
        self.actions.append("fulfill")
        self.fulfilled = kwargs


class FakeNavResponse:
    def __init__(self, status):
        self.status = status


class FakeCDP:
    def __init__(self, payload):
        self.calls = []
        self.payload = payload

    def send(self, method, params=None, timeout=None):
        self.calls.append((method, params, timeout))
        return {"data": self.payload}


class FakePage:
    def __init__(self, cdp):
        self._cdp = cdp
        self.url = ""
        self.goto_calls = []
        self.context = None
        self.handlers = {}
        self.closed = False

    def route(self, pattern, handler):
        self.handlers["route"] = (pattern, handler)

    def on(self, event, handler):
        self.handlers[event] = handler

    def goto(self, url, **kwargs):
        self.goto_calls.append((url, kwargs))
        _pattern, handler = self.handlers["route"]
        route = FakeRoute(url, navigation=True)
        handler(route)
        if route.actions[-1:] == ["abort"]:
            raise chromium_mod().Error("net::ERR_ABORTED")
        self.url = url
        status = 200
        if route.fulfilled:
            status = route.fulfilled.get("status", 200)
        return FakeNavResponse(status)

    def set_default_timeout(self, ms):
        pass


class FakeContext:
    def __init__(self, page):
        self._page = page
        page.context = self
        self.closed = False
        self.timeout = None

    def new_page(self):
        return self._page

    def new_cdp_session(self, page):
        return page._cdp

    def close(self):
        self.closed = True

    def set_default_timeout(self, ms):
        self.timeout = ms


class FakeBrowser:
    def __init__(self, payload):
        self.cdp = FakeCDP(payload)
        self.page = FakePage(self.cdp)
        self.context = FakeContext(self.page)
        self.context_kwargs = None
        self.closed = False

    def new_context(self, **kwargs):
        self.context_kwargs = kwargs
        return self.context

    def close(self):
        self.closed = True


def serve_fixture():
    html = SHELL.read_bytes()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(html)))
            self.end_headers()
            self.wfile.write(html)

        def log_message(self, format, *args):
            return

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


def test_missing_url_fails_and_keeps_the_item(core):
    item = ItemFactory(type="capture.link")
    archive(core, item)
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id).exists()
    assert not Item.objects.filter(type="capture.file").exists()


def test_public_https_url_stores_mhtml_blob(core, blob_dir, monkeypatch):
    timeouts = []
    patch_capture(monkeypatch, returns_mhtml(timeout_seen=timeouts))
    item = archive(core, make_link(PAGE))
    files = derived_files(item)
    assert len(files) == 1
    derived = files[0]
    values = derived.current_values()
    sha = values["$blob"]
    assert values["$key"] == "mhtml"
    assert Blob.objects.get(pk=sha).content_type == "multipart/related"
    assert (blob_dir / sha[:2] / sha).read_bytes() == MHTML
    assert (
        Link.objects.live()
        .filter(source=derived, target=item, rel="derived_from")
        .exists()
    )
    url_claim = Claim.objects.get(item=item, path="url")
    assert url_claim.value == PAGE
    assert url_claim.rank == Claim.Rank.PREFERRED
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "ok"
    assert timeouts == [30]


def test_js_rendered_local_fixture_stores_rendered_text(core, blob_dir, monkeypatch):
    if not chromium_installed():
        pytest.skip("Playwright Chromium is not installed")
    assert "hallway lamp" not in SHELL_HTML
    server = serve_fixture()
    try:
        url = f"http://127.0.0.1:{server.server_address[1]}/"
        monkeypatch.setattr(archive_mod(), "require_public_http", lambda u: None)
        monkeypatch.setattr(chromium_mod(), "require_public_http", lambda u: None)
        item = archive(core, make_link(url))
        files = derived_files(item)
        assert len(files) == 1
        sha = files[0].current_values()["$blob"]
        stored = (blob_dir / sha[:2] / sha).read_bytes()
        assert b"hallway lamp" in stored
        act = Activity.objects.get(agent="archive@2", subject=item)
        assert act.outcome == "ok"
    finally:
        server.shutdown()


def test_same_url_twice_gets_two_archive_items(core, blob_dir, monkeypatch):
    patch_capture(monkeypatch, returns_mhtml())
    first = archive(core, make_link(PAGE))
    second = archive(core, make_link(PAGE))
    assert first.id != second.id
    first_files = derived_files(first)
    second_files = derived_files(second)
    assert len(first_files) == 1
    assert len(second_files) == 1
    assert first_files[0].id != second_files[0].id


def test_rederive_same_version_skips_a_second_fetch(core, blob_dir, monkeypatch):
    calls = []

    def capture(url, *, timeout):
        calls.append(url)
        return MHTML

    patch_capture(monkeypatch, capture)
    item = archive(core, make_link(PAGE))
    first = derived_files(item)[0]
    archive(core, item)
    assert calls == [PAGE]
    assert [row.id for row in derived_files(item)] == [first.id]
    agents = list(
        Activity.objects.filter(kind="derive", subject=item, outcome="ok")
        .order_by("started")
        .values_list("agent", flat=True)
    )
    assert agents == ["archive@2"]


def test_v2_runs_when_v1_already_ok(core, blob_dir, monkeypatch):
    patch_capture(monkeypatch, returns_mhtml())
    item = make_link(PAGE)
    ActivityFactory(
        kind="derive",
        agent="archive@1",
        subject=item,
        outcome=Activity.Outcome.OK,
    )
    archive(core, item)
    files = derived_files(item)
    assert len(files) == 1
    act = Activity.objects.get(agent="archive@2", subject=item, kind="derive")
    assert act.outcome == "ok"


@pytest.mark.parametrize(
    "url",
    [
        "file:///etc/passwd",
        "data:text/html,hi",
        "http://127.0.0.1/",
        "http://10.0.0.1/",
        "http://100.64.0.1/",
        "http://169.254.1.1/",
    ],
)
def test_private_urls_fail_without_launching(core, monkeypatch, url):
    launched = []

    @contextmanager
    def fake_launch():
        launched.append(True)
        yield FakeBrowser(MHTML.decode())

    monkeypatch.setattr(chromium_mod(), "launch_chromium", fake_launch)
    item = archive(core, make_link(url))
    assert launched == []
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


@pytest.mark.parametrize("location", ["http://127.0.0.1/", "http://100.64.0.1/"])
def test_redirect_to_private_ip_is_aborted(core, location):
    chromium = chromium_mod()
    blocked = []
    route = FakeRoute(location, navigation=True)
    chromium.handle_route(route, blocked, timeout=5)
    assert route.actions == ["abort"]
    assert blocked == [location]


def test_redirect_location_to_private_ip_is_aborted(core, monkeypatch):
    chromium = chromium_mod()

    def hop(url, *, method, headers, body, timeout):
        return 302, {"Location": "http://127.0.0.1/secret"}, b""

    monkeypatch.setattr(chromium, "fetch_hop", hop)
    blocked = []
    route = FakeRoute(PAGE, navigation=True)
    chromium.handle_route(route, blocked, timeout=5)
    assert route.actions == ["abort"]
    assert blocked == ["http://127.0.0.1/secret"]


def test_public_navigation_is_fulfilled(core, monkeypatch):
    chromium = chromium_mod()
    patch_hop(monkeypatch, body=b"ok")
    blocked = []
    route = FakeRoute(PAGE, navigation=True)
    chromium.handle_route(route, blocked, timeout=5)
    assert route.actions == ["fulfill"]
    assert route.fulfilled["status"] == 200
    assert route.fulfilled["body"] == b"ok"
    assert blocked == []
    assert "continue" not in route.actions


def test_file_subresource_is_aborted_without_failing_the_page(core):
    chromium = chromium_mod()
    blocked = []
    route = FakeRoute("file:///etc/passwd", navigation=False)
    chromium.handle_route(route, blocked, timeout=5)
    assert route.actions == ["abort"]
    assert blocked == []


def test_oversize_archive_fails_without_storing(core, blob_dir, monkeypatch):
    patch_capture(monkeypatch, returns_mhtml(b"x" * (20 * 1024 * 1024 + 1)))
    item = archive(core, make_link(PAGE))
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


@pytest.mark.parametrize("fake", [timed_out, capture_failed])
def test_fetch_failures_fail_derive_and_keep_the_link(core, monkeypatch, fake):
    patch_capture(monkeypatch, fake)
    item = archive(core, make_link(PAGE))
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


def test_http_error_status_fails_without_snapshot(core, blob_dir, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    @contextmanager
    def fake_launch():
        yield browser

    def hop(url, *, method, headers, body, timeout):
        return 404, {"Content-Type": "text/html"}, b"nope"

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    monkeypatch.setattr(chromium, "fetch_hop", hop)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    item = archive(core, make_link(PAGE))
    act = Activity.objects.get(agent="archive@2", subject=item)
    assert act.outcome == "failed"
    assert browser.cdp.calls == []
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


def test_cdp_snapshot_requests_mhtml(core):
    chromium = chromium_mod()
    cdp = FakeCDP(MHTML.decode())
    page = FakePage(cdp)
    page.context = FakeContext(page)
    data = chromium._cdp_mhtml(page, timeout_ms=12000)
    assert cdp.calls == [("Page.captureSnapshot", {"format": "mhtml"}, 12000)]
    assert data == MHTML.decode()


def test_capture_mhtml_uses_bundled_headless_chromium(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    patch_hop(monkeypatch)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    data = chromium.capture_mhtml(PAGE, timeout=30)
    assert data == MHTML
    assert browser.context_kwargs == {"accept_downloads": False}
    assert "storage_state" not in browser.context_kwargs
    assert "extra_http_headers" not in browser.context_kwargs
    assert browser.page.goto_calls == [(PAGE, {"timeout": 30000, "wait_until": "load"})]
    assert browser.cdp.calls == [("Page.captureSnapshot", {"format": "mhtml"}, 30000)]
    assert browser.context.closed
    assert "download" in browser.page.handlers


def test_launch_chromium_disables_non_proxied_webrtc(monkeypatch):
    chromium = chromium_mod()
    seen = {}

    class FakeLaunched:
        def close(self):
            pass

    class FakeChromiumType:
        def launch(self, **kwargs):
            seen.update(kwargs)
            return FakeLaunched()

    class FakePlaywright:
        chromium = FakeChromiumType()

        def stop(self):
            pass

    class FakeManager:
        _client = None

        def start(self):
            return FakePlaywright()

    monkeypatch.setattr(chromium, "sync_playwright", lambda: FakeManager())
    with chromium.launch_chromium():
        pass
    assert seen["headless"] is True
    assert seen["args"] == chromium.LAUNCH_ARGS
    assert "--force-webrtc-ip-handling-policy=disable_non_proxied_udp" in seen["args"]
