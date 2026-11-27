import http.client
import ipaddress
import os
import socket
import sys
import time
from contextlib import contextmanager
from email.message import Message
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
    if os.environ.get("CI"):
        return True
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


class FakeAPIResponse:
    def __init__(self, status=200, headers=None, body=b"<html></html>"):
        self.status = status
        self.headers = {
            key.lower(): value for key, value in dict(headers or {}).items()
        }
        self._body = body

    def body(self):
        return self._body


class FakeRoute:
    def __init__(self, url, *, navigation=False, main=True, fetch_response=None):
        self.request = FakeRequest(url, navigation=navigation, main=main)
        self.actions = []
        self.fulfilled = None
        self.fetch_kwargs = None
        self._fetch_response = fetch_response or FakeAPIResponse()

    def continue_(self):
        self.actions.append("continue")

    def abort(self):
        self.actions.append("abort")

    def fetch(self, **kwargs):
        self.actions.append("fetch")
        self.fetch_kwargs = kwargs
        return self._fetch_response

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
    def __init__(self, cdp, fetch_response=None):
        self._cdp = cdp
        self.url = ""
        self.goto_calls = []
        self.context = None
        self.handlers = {}
        self.closed = False
        self.fetch_response = fetch_response or FakeAPIResponse()

    def route(self, pattern, handler):
        self.handlers["route"] = (pattern, handler)

    def on(self, event, handler):
        self.handlers[event] = handler

    def goto(self, url, **kwargs):
        self.goto_calls.append((url, kwargs))
        _pattern, handler = self.context.handlers["route"]
        route = FakeRoute(url, navigation=True, fetch_response=self.fetch_response)
        handler(route)
        if route.actions[-1:] == ["abort"]:
            raise chromium_mod().Error("net::ERR_ABORTED")
        self.url = url
        status = 200
        if route.fulfilled:
            if route.fulfilled.get("response") is not None:
                status = route.fulfilled["response"].status
            else:
                status = route.fulfilled.get("status", 200)
        return FakeNavResponse(status)

    def close(self):
        self.closed = True

    def set_default_timeout(self, ms):
        pass


class FakeContext:
    def __init__(self, page):
        self._page = page
        page.context = self
        self.closed = False
        self.timeout = None
        self.handlers = {}

    def route(self, pattern, handler):
        self.handlers["route"] = (pattern, handler)

    def route_web_socket(self, pattern, handler):
        self.handlers["websocket"] = (pattern, handler)

    def on(self, event, handler):
        self.handlers[event] = handler

    def new_page(self):
        return self._page

    def new_cdp_session(self, page):
        return page._cdp

    def close(self):
        self.closed = True

    def set_default_timeout(self, ms):
        self.timeout = ms


class FakeBrowser:
    def __init__(self, payload, fetch_response=None):
        self.cdp = FakeCDP(payload)
        self.page = FakePage(self.cdp, fetch_response=fetch_response)
        self.context = FakeContext(self.page)
        self.context_kwargs = None
        self.closed = False

    def new_context(self, **kwargs):
        self.context_kwargs = kwargs
        return self.context

    def close(self):
        self.closed = True
        self.context.close()


class FakeProc:
    pid = 4242


class FakeTransport:
    _proc = FakeProc()


class FakeClient:
    _transport = FakeTransport()


class FakeLaunched:
    def close(self):
        pass


class FakeChromiumType:
    def __init__(self, seen):
        self._seen = seen

    def launch(self, **kwargs):
        self._seen.update(kwargs)
        return FakeLaunched()


class FakePlaywright:
    def __init__(self, seen):
        self.chromium = FakeChromiumType(seen)
        self.stopped = False

    def stop(self):
        self.stopped = True


class FakeManager:
    def __init__(self, playwright, *, pid=4242):
        self._playwright = playwright
        if pid is None:
            self._client = None
        else:
            proc = FakeProc()
            proc.pid = pid
            transport = FakeTransport()
            transport._proc = proc
            client = FakeClient()
            client._transport = transport
            self._client = client

    def start(self):
        return self._playwright


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
    if not os.environ.get("CI") and not chromium_installed():
        pytest.skip("Playwright Chromium is not installed")
    assert "hallway lamp" not in SHELL_HTML
    server = serve_fixture()
    try:
        url = f"http://127.0.0.1:{server.server_address[1]}/"
        monkeypatch.setattr(archive_mod(), "require_public_http", lambda u: "127.0.0.1")
        monkeypatch.setattr(
            chromium_mod(), "require_public_http", lambda u: "127.0.0.1"
        )
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
    route = FakeRoute(location, navigation=True)
    with pytest.raises(FetchError, match="private url"):
        chromium.handle_route(route, timeout=5)
    assert route.actions == ["abort"]


def test_redirect_location_to_private_ip_is_aborted(core, monkeypatch):
    chromium = chromium_mod()

    def hop(url, *, method, headers, body, timeout):
        return 302, {"Location": "http://127.0.0.1/secret"}, b""

    monkeypatch.setattr(chromium, "fetch_hop", hop)
    route = FakeRoute(PAGE, navigation=True)
    with pytest.raises(FetchError, match="private url"):
        chromium.handle_route(route, timeout=5)
    assert route.actions == ["abort"]
    assert "continue" not in route.actions


def test_public_navigation_is_fulfilled(core, monkeypatch):
    chromium = chromium_mod()
    patch_hop(monkeypatch, body=b"ok")
    route = FakeRoute(PAGE, navigation=True)
    chromium.handle_route(route, timeout=5)
    assert route.actions == ["fulfill"]
    assert route.fulfilled["body"] == b"ok"
    assert "continue" not in route.actions


def test_file_subresource_is_aborted_without_failing_the_page(core):
    chromium = chromium_mod()
    route = FakeRoute("file:///etc/passwd", navigation=False)
    chromium.handle_route(route, timeout=5)
    assert route.actions == ["abort"]


def test_data_url_is_aborted_not_continued(core):
    chromium = chromium_mod()
    route = FakeRoute("data:text/html,hi", navigation=False)
    chromium.handle_route(route, timeout=5)
    assert route.actions == ["abort"]
    assert "continue" not in route.actions


def test_oversize_hop_fails_main_navigation(core, monkeypatch):
    chromium = chromium_mod()
    patch_hop(monkeypatch, body=b"x" * (chromium.MAX_HOP_BYTES + 1))
    route = FakeRoute(PAGE, navigation=True)
    with pytest.raises(chromium.CaptureError, match="archive too large"):
        chromium.handle_route(route, timeout=5)


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
    browser = FakeBrowser(
        MHTML.decode(),
        fetch_response=FakeAPIResponse(404, {"Content-Type": "text/html"}, b"nope"),
    )

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    monkeypatch.setattr(
        chromium,
        "fetch_hop",
        lambda url, *, method, headers, body, timeout: (
            404,
            {"Content-Type": "text/html"},
            b"nope",
        ),
    )
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
    assert browser.context_kwargs == {
        "accept_downloads": False,
        "service_workers": "block",
    }
    assert "storage_state" not in browser.context_kwargs
    assert "extra_http_headers" not in browser.context_kwargs
    assert browser.page.goto_calls == [(PAGE, {"timeout": 30000, "wait_until": "load"})]
    assert browser.cdp.calls == [("Page.captureSnapshot", {"format": "mhtml"}, 30000)]
    assert "download" in browser.page.handlers
    assert "route" in browser.context.handlers
    assert "websocket" in browser.context.handlers
    assert "page" in browser.context.handlers
    assert "route" not in browser.page.handlers


def test_launch_chromium_disables_non_proxied_webrtc(monkeypatch):
    chromium = chromium_mod()
    seen = {}
    playwright = FakePlaywright(seen)
    monkeypatch.setattr(chromium, "sync_playwright", lambda: FakeManager(playwright))
    with chromium.launch_chromium():
        pass
    assert seen["headless"] is True
    assert seen["args"] == chromium.LAUNCH_ARGS
    assert "--force-webrtc-ip-handling-policy=disable_non_proxied_udp" in seen["args"]
    assert playwright.stopped


def test_launch_aborts_without_driver_pid(core, monkeypatch):
    chromium = chromium_mod()
    seen = {}
    playwright = FakePlaywright(seen)
    monkeypatch.setattr(
        chromium, "sync_playwright", lambda: FakeManager(playwright, pid=None)
    )
    with pytest.raises(chromium.CaptureError, match="chromium driver pid missing"):
        with chromium.launch_chromium():
            pass
    assert seen == {}
    assert playwright.stopped


def test_launch_failure_still_stops_playwright(monkeypatch):
    chromium = chromium_mod()
    seen = {}

    class Boom(FakeChromiumType):
        def launch(self, **kwargs):
            raise RuntimeError("nope")

    playwright = FakePlaywright(seen)
    playwright.chromium = Boom(seen)
    monkeypatch.setattr(chromium, "sync_playwright", lambda: FakeManager(playwright))
    with pytest.raises(RuntimeError, match="nope"):
        with chromium.launch_chromium():
            pass
    assert playwright.stopped


def test_fetch_hop_pins_validated_ip_against_rebinding(monkeypatch):
    chromium = chromium_mod()
    connected = []
    seen_host = {"n": 0}

    def fake_getaddrinfo(host, port, *args, **kwargs):
        try:
            ipaddress.ip_address(host)
            return [(socket.AF_INET, socket.SOCK_STREAM, 0, "", (host, port or 0))]
        except ValueError:
            pass
        seen_host["n"] += 1
        if seen_host["n"] == 1:
            return [
                (
                    socket.AF_INET,
                    socket.SOCK_STREAM,
                    0,
                    "",
                    ("93.184.216.34", port or 0),
                )
            ]
        return [(socket.AF_INET, socket.SOCK_STREAM, 0, "", ("127.0.0.1", port or 0))]

    def fake_create_connection(address, timeout=None, source_address=None):
        connected.append(address)
        raise OSError("stop")

    monkeypatch.setattr("trove.services.public_url.getaddrinfo", fake_getaddrinfo)
    monkeypatch.setattr(
        "trove.services.public_http.socket.create_connection", fake_create_connection
    )
    with pytest.raises((OSError, FetchError, chromium.CaptureError)):
        chromium.fetch_hop(PAGE, method="GET", headers={}, body=None, timeout=5)
    assert connected
    assert connected[0][0] == "93.184.216.34"
    assert all(addr[0] != "127.0.0.1" for addr in connected)


def test_new_context_blocks_service_workers(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    patch_hop(monkeypatch)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    chromium.capture_mhtml(PAGE, timeout=30)
    assert browser.context_kwargs["service_workers"] == "block"


def test_context_websocket_route_closes_loopback(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    patch_hop(monkeypatch)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    chromium.capture_mhtml(PAGE, timeout=30)
    pattern, handler = browser.context.handlers["websocket"]
    assert pattern == "**/*"

    class FakeWebSocket:
        def __init__(self):
            self.url = "ws://127.0.0.1/socket"
            self.closed = False

        def close(self):
            self.closed = True

    ws = FakeWebSocket()
    handler(ws)
    assert ws.closed


def test_archive_plugin_never_calls_route_continue():
    root = Path(__file__).resolve().parents[1] / "plugins" / "archive"
    for path in root.glob("*.py"):
        src = path.read_text()
        assert "continue_()" not in src, path
        assert "route.continue_" not in src, path


def test_routes_attach_to_context_not_page(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    patch_hop(monkeypatch)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    chromium.capture_mhtml(PAGE, timeout=30)
    assert "route" in browser.context.handlers
    assert "route" not in browser.page.handlers


def test_popup_pages_are_closed(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())
    popup = FakePage(FakeCDP(""))
    popup.context = browser.context

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    patch_hop(monkeypatch)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    chromium.capture_mhtml(PAGE, timeout=30)
    browser.context.handlers["page"](popup)
    assert popup.closed


def test_hung_close_kills_driver(monkeypatch):
    chromium = chromium_mod()
    killed = []
    monkeypatch.setattr(chromium, "CLOSE_WAIT_S", 0.05)
    monkeypatch.setattr(chromium, "_kill_pid", lambda pid: killed.append(pid))

    class HungBrowser:
        def close(self):
            time.sleep(0.2)

    class Stopped:
        def stop(self):
            pass

    chromium._close_chromium(HungBrowser(), Stopped(), 4242)
    assert killed == [4242]


class FakeHopResponse:
    def __init__(self, status, headers, body=b""):
        self.status = status
        self.headers = headers
        self._body = body

    def read(self, n=-1):
        return self._body

    def __enter__(self):
        return self

    def __exit__(self, *args):
        return False


def test_multiple_location_headers_are_aborted(core, monkeypatch):
    chromium = chromium_mod()
    headers = Message()
    headers.add_header("Location", "https://example.com/a")
    headers.add_header("Location", "https://example.com/b")
    monkeypatch.setattr(chromium, "require_public_http", lambda url: "93.184.216.34")
    monkeypatch.setattr(
        chromium,
        "open_hop",
        lambda request, timeout=None: FakeHopResponse(302, headers),
    )
    route = FakeRoute(PAGE, navigation=True)
    with pytest.raises(FetchError, match="multiple location headers"):
        chromium.handle_route(route, timeout=5)
    assert route.actions == ["abort"]


def test_fetch_hop_http_exception_is_fetch_error(core, monkeypatch):
    chromium = chromium_mod()
    monkeypatch.setattr(chromium, "require_public_http", lambda url: "93.184.216.34")

    def boom(request, timeout=None):
        raise http.client.HTTPException("incomplete read")

    monkeypatch.setattr(chromium, "open_hop", boom)
    with pytest.raises(FetchError, match="incomplete read"):
        chromium.fetch_hop(PAGE, method="GET", headers={}, body=None, timeout=5)


def test_on_route_wraps_unexpected_errors(core, monkeypatch):
    chromium = chromium_mod()
    browser = FakeBrowser(MHTML.decode())

    def boom(route, *, timeout):
        raise RuntimeError("driver exploded")

    @contextmanager
    def fake_launch():
        yield browser

    monkeypatch.setattr(chromium, "launch_chromium", fake_launch)
    monkeypatch.setattr(chromium, "handle_route", boom)
    monkeypatch.setattr(chromium.time, "monotonic", lambda: 1000.0)
    with pytest.raises(chromium.CaptureError, match="driver exploded"):
        chromium.capture_mhtml(PAGE, timeout=30)


def test_fetch_hop_strips_accept_encoding(core, monkeypatch):
    chromium = chromium_mod()
    seen = {}
    headers = Message()
    headers["Content-Type"] = "text/html"

    def open_hop(request, timeout=None):
        seen["request"] = request
        return FakeHopResponse(200, headers, b"ok")

    monkeypatch.setattr(chromium, "require_public_http", lambda url: "93.184.216.34")
    monkeypatch.setattr(chromium, "open_hop", open_hop)
    status, response_headers, body = chromium.fetch_hop(
        PAGE,
        method="GET",
        headers={"Accept-Encoding": "gzip", "Accept": "text/html"},
        body=None,
        timeout=5,
    )
    assert status == 200
    assert body == b"ok"
    assert seen["request"].get_header("Accept-encoding") is None
    assert seen["request"].get_header("Accept") == "text/html"


def test_chromium_installed_does_not_skip_on_ci(monkeypatch):
    monkeypatch.setenv("CI", "true")
    assert chromium_installed() is True
