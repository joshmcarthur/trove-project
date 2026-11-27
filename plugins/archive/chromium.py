"""Load a public URL in bundled Chromium and snapshot it as MHTML."""

from __future__ import annotations

import os
import signal
import threading
import time
import urllib.request
from contextlib import contextmanager
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin, urlparse
from urllib.request import Request

from playwright.sync_api import (
    Error,
    sync_playwright,
)
from playwright.sync_api import (
    TimeoutError as PlaywrightTimeout,
)

from trove.services.fetch_error import FetchError
from trove.services.public_url import require_public_http

INTERNAL_SCHEMES = {"about", "blob", "chrome", "chrome-untrusted", "devtools"}
SNAPSHOT_METHOD = "Page.captureSnapshot"
SNAPSHOT_PARAMS = {"format": "mhtml"}
LAUNCH_ARGS = ["--force-webrtc-ip-handling-policy=disable_non_proxied_udp"]
CLOSE_WAIT_S = 5
MAX_HOP_BYTES = 20 * 1024 * 1024
HOP_DROP_HEADERS = {
    "connection",
    "content-encoding",
    "content-length",
    "keep-alive",
    "proxy-connection",
    "transfer-encoding",
}
HOP_SKIP_REQUEST_HEADERS = {
    "connection",
    "content-length",
    "host",
    "keep-alive",
    "transfer-encoding",
}


class CaptureError(Exception):
    pass


def _hop_opener():
    opener = urllib.request.OpenerDirector()
    for handler in (
        urllib.request.UnknownHandler(),
        urllib.request.HTTPHandler(),
        urllib.request.HTTPSHandler(),
        urllib.request.HTTPDefaultErrorHandler(),
        urllib.request.HTTPErrorProcessor(),
    ):
        opener.add_handler(handler)
    return opener


_HOP_OPENER = _hop_opener()


def _driver_pid(manager) -> int | None:
    client = getattr(manager, "_client", None)
    transport = getattr(client, "_transport", None)
    proc = getattr(transport, "_proc", None)
    pid = getattr(proc, "pid", None)
    return pid if isinstance(pid, int) else None


def _kill_pid(pid: int | None) -> None:
    if not pid:
        return
    try:
        os.kill(pid, signal.SIGKILL)
    except OSError:
        pass


def _close_chromium(browser, playwright, pid: int | None) -> None:
    timer = threading.Timer(CLOSE_WAIT_S, _kill_pid, args=(pid,))
    timer.daemon = True
    timer.start()
    try:
        try:
            browser.close()
        except Exception:
            pass
        try:
            playwright.stop()
        except Exception:
            pass
    finally:
        timer.cancel()


@contextmanager
def launch_chromium():
    manager = sync_playwright()
    playwright = manager.start()
    pid = _driver_pid(manager)
    browser = playwright.chromium.launch(headless=True, args=LAUNCH_ARGS)
    try:
        yield browser
    finally:
        _close_chromium(browser, playwright, pid)


def capture_mhtml(url: str, *, timeout: int) -> bytes:
    try:
        with launch_chromium() as browser:
            return _snapshot(browser, url, timeout_ms=timeout * 1000)
    except PlaywrightTimeout as exc:
        raise CaptureError("timed out") from exc
    except FetchError:
        raise
    except Error as exc:
        message = str(exc)
        if "Executable doesn't exist" in message:
            raise CaptureError("chromium is not installed") from exc
        raise CaptureError(message or "capture failed") from exc


def _remaining_ms(deadline: float) -> int:
    left = deadline - time.monotonic()
    if left <= 0:
        raise CaptureError("timed out")
    return max(1, int(left * 1000))


def _remaining_s(deadline: float) -> float:
    left = deadline - time.monotonic()
    if left <= 0:
        raise CaptureError("timed out")
    return left


def _snapshot(browser, url: str, *, timeout_ms: int) -> bytes:
    context = browser.new_context(accept_downloads=False)
    blocked_nav: list[str] = []
    failure: list[BaseException] = []
    deadline = time.monotonic() + timeout_ms / 1000
    try:
        page = context.new_page()
        context.set_default_timeout(timeout_ms)
        page.on("download", lambda download: download.cancel())

        def on_route(route):
            try:
                handle_route(route, blocked_nav, timeout=_remaining_s(deadline))
            except (CaptureError, FetchError) as exc:
                failure.append(exc)
                try:
                    route.abort()
                except Exception:
                    pass

        page.route("**/*", on_route)
        try:
            response = page.goto(
                url, timeout=_remaining_ms(deadline), wait_until="load"
            )
        except Error:
            if failure:
                raise failure[0] from None
            if blocked_nav:
                raise FetchError("private url") from None
            raise
        if failure:
            raise failure[0]
        if blocked_nav:
            raise FetchError("private url")
        if response is None:
            raise CaptureError("navigation failed")
        if response.status >= 400:
            raise FetchError(f"HTTP {response.status}")
        require_public_http(page.url)
        data = _cdp_mhtml(page, timeout_ms=_remaining_ms(deadline))
    finally:
        context.close()
    if isinstance(data, bytes):
        return data
    return data.encode()


def _cdp_mhtml(page, *, timeout_ms: int) -> str:
    session = page.context.new_cdp_session(page)
    result = session.send(SNAPSHOT_METHOD, SNAPSHOT_PARAMS, timeout=timeout_ms)
    data = result.get("data")
    if not data:
        raise CaptureError("empty snapshot")
    return data


def handle_route(route, blocked_nav: list[str], *, timeout: float) -> None:
    request = route.request
    url = request.url
    main = _is_main_navigation(request)
    if main:
        if not _is_public_http(url):
            blocked_nav.append(url)
            route.abort()
            return
        _fulfill_hop(route, url, blocked_nav, timeout=timeout, main=True)
        return
    scheme = urlparse(url).scheme.lower()
    if scheme in INTERNAL_SCHEMES or scheme == "data":
        route.continue_()
        return
    if not _subresource_allowed(url):
        route.abort()
        return
    if scheme in {"ws", "wss"}:
        route.abort()
        return
    _fulfill_hop(route, url, blocked_nav, timeout=timeout, main=False)


def _fulfill_hop(route, url, blocked_nav, *, timeout, main) -> None:
    try:
        status, headers, body = fetch_hop(
            url,
            method=getattr(route.request, "method", "GET") or "GET",
            headers=dict(getattr(route.request, "headers", None) or {}),
            body=_request_body(route.request),
            timeout=timeout,
        )
    except FetchError:
        if main:
            raise
        route.abort()
        return
    if 300 <= status < 400:
        location = _header(headers, "location")
        if location:
            next_url = urljoin(url, location)
            try:
                require_public_http(next_url)
            except FetchError:
                if main:
                    blocked_nav.append(next_url)
                route.abort()
                return
    if len(body) > MAX_HOP_BYTES:
        if main:
            raise CaptureError("archive too large")
        route.abort()
        return
    route.fulfill(status=status, headers=headers, body=body)


def _request_body(request) -> bytes | None:
    buffer = getattr(request, "post_data_buffer", None)
    if buffer:
        return bytes(buffer)
    data = getattr(request, "post_data", None)
    if data is None:
        return None
    if isinstance(data, bytes):
        return data
    return data.encode()


def _header(headers: dict[str, str], name: str) -> str | None:
    for key, value in headers.items():
        if key.lower() == name:
            return value
    return None


def fetch_hop(
    url: str, *, method: str, headers: dict, body: bytes | None, timeout: float
) -> tuple[int, dict[str, str], bytes]:
    require_public_http(url)
    outgoing = {}
    for key, value in headers.items():
        lowered = key.lower()
        if lowered in HOP_SKIP_REQUEST_HEADERS or key.startswith(":"):
            continue
        outgoing[key] = value
    request = Request(url, data=body, method=method, headers=outgoing)
    try:
        with _HOP_OPENER.open(request, timeout=timeout) as response:
            return _hop_result(response)
    except HTTPError as exc:
        try:
            return _hop_result(exc)
        finally:
            exc.close()
    except TimeoutError as exc:
        raise CaptureError("timed out") from exc
    except URLError as exc:
        if isinstance(exc.reason, TimeoutError):
            raise CaptureError("timed out") from exc
        raise FetchError(str(exc) or "fetch failed") from exc
    except OSError as exc:
        raise FetchError(str(exc) or "fetch failed") from exc


def _hop_result(response) -> tuple[int, dict[str, str], bytes]:
    status = getattr(response, "status", None) or response.getcode()
    headers = {}
    for key, value in response.headers.items():
        if key.lower() in HOP_DROP_HEADERS:
            continue
        headers[key] = value
    body = response.read(MAX_HOP_BYTES + 1)
    return int(status), headers, body


def _is_main_navigation(request) -> bool:
    if not request.is_navigation_request():
        return False
    frame = getattr(request, "frame", None)
    if frame is None:
        return True
    return getattr(frame, "parent_frame", None) is None


def _is_public_http(url: str) -> bool:
    try:
        require_public_http(url)
        return True
    except FetchError:
        return False


def _subresource_allowed(url: str) -> bool:
    scheme = urlparse(url).scheme.lower()
    if scheme in INTERNAL_SCHEMES or scheme == "data":
        return True
    if scheme in {"ws", "wss"}:
        return _is_public_http(_http_url_for_policy(url))
    if scheme in {"http", "https"}:
        return _is_public_http(url)
    return False


def _http_url_for_policy(url: str) -> str:
    parsed = urlparse(url)
    scheme = parsed.scheme.lower()
    rest = url.split("://", 1)[1] if "://" in url else url
    if scheme == "ws":
        return f"http://{rest}"
    if scheme == "wss":
        return f"https://{rest}"
    return url
