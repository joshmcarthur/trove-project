"""Load a public URL in bundled Chromium and snapshot it as MHTML."""

from __future__ import annotations

import os
import signal
import threading
import time
from contextlib import contextmanager
from urllib.parse import urljoin, urlparse

from playwright.sync_api import Error, Request, Route, sync_playwright
from playwright.sync_api import TimeoutError as PlaywrightTimeout

from trove.services.fetch_error import FetchError
from trove.services.public_url import require_public_http

INTERNAL_SCHEMES = {"about", "blob", "chrome", "chrome-untrusted", "devtools"}
SNAPSHOT_METHOD = "Page.captureSnapshot"
SNAPSHOT_PARAMS = {"format": "mhtml"}
LAUNCH_ARGS = ["--force-webrtc-ip-handling-policy=disable_non_proxied_udp"]
CLOSE_WAIT_S = 5
MAX_HOP_BYTES = 20 * 1024 * 1024
HOP_STRIP_HEADERS = {"authorization"}


class CaptureError(Exception):
    pass


def _driver_pid(manager) -> int | None:
    client = getattr(manager, "_client", None)
    transport = getattr(client, "_transport", None)
    proc = getattr(transport, "_proc", None)
    pid = getattr(proc, "pid", None)
    return pid if isinstance(pid, int) else None


def _kill_pid(pid: int) -> None:
    try:
        os.kill(pid, signal.SIGKILL)
    except OSError:
        pass


def _close_chromium(browser, playwright, pid: int) -> None:
    timer = threading.Timer(CLOSE_WAIT_S, _kill_pid, args=(pid,))
    timer.daemon = True
    timer.start()
    try:
        if browser is not None:
            for context in list(browser.contexts):
                try:
                    context.close()
                except Exception:
                    pass
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
    if pid is None:
        try:
            playwright.stop()
        except Exception:
            pass
        raise CaptureError("chromium driver pid missing")
    browser = None
    try:
        browser = playwright.chromium.launch(headless=True, args=LAUNCH_ARGS)
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


def _route_failure(exc: BaseException) -> BaseException:
    if isinstance(exc, (CaptureError, FetchError)):
        return exc
    if isinstance(exc, (PlaywrightTimeout, TimeoutError)):
        return CaptureError("timed out")
    return CaptureError(str(exc) or "fetch failed")


def _snapshot(browser, url: str, *, timeout_ms: int) -> bytes:
    context = browser.new_context(
        accept_downloads=False,
        service_workers="block",
    )
    failure: list[BaseException] = []
    deadline = time.monotonic() + timeout_ms / 1000
    page = context.new_page()
    context.set_default_timeout(timeout_ms)
    page.on("download", lambda download: download.cancel())

    def on_route(route):
        try:
            handle_route(route, timeout=_remaining_s(deadline))
        except (
            CaptureError,
            FetchError,
            PlaywrightTimeout,
            Error,
            OSError,
            TimeoutError,
        ) as exc:
            try:
                route.abort()
            except Exception:
                pass
            if _is_main_navigation(route.request):
                failure.append(_route_failure(exc))

    context.route("**/*", on_route)
    context.route_web_socket("**/*", lambda ws: ws.close())

    def on_page(opened):
        if opened is not page:
            try:
                opened.close()
            except Exception:
                pass

    context.on("page", on_page)
    try:
        response = page.goto(url, timeout=_remaining_ms(deadline), wait_until="load")
    except Error:
        if failure:
            raise failure[0] from None
        raise
    if failure:
        raise failure[0]
    if response is None:
        raise CaptureError("navigation failed")
    if response.status >= 400:
        raise FetchError(f"HTTP {response.status}")
    require_public_http(page.url)
    data = _cdp_mhtml(page, timeout_ms=_remaining_ms(deadline))
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


def _hop_headers(request: Request) -> dict[str, str]:
    return {
        key: value
        for key, value in request.headers.items()
        if key.lower() not in HOP_STRIP_HEADERS
    }


def _locations(response) -> list[str]:
    return [
        row["value"]
        for row in response.headers_array
        if str(row.get("name", "")).lower() == "location"
    ]


def handle_route(route: Route, *, timeout: float) -> None:
    url = route.request.url
    main = _is_main_navigation(route.request)
    scheme = urlparse(url).scheme.lower()
    # about/blob/chrome/data are not HTTP hops. route.fetch cannot load them.
    # continue_() lets Chromium keep document blobs and inline data URLs.
    if not main and scheme in INTERNAL_SCHEMES | {"data"}:
        route.continue_()
        return
    if scheme in {"ws", "wss"} or not _is_public_http(url):
        if main:
            raise FetchError("private url")
        route.abort()
        return
    try:
        response = route.fetch(
            headers=_hop_headers(route.request),
            max_redirects=0,
            timeout=timeout * 1000,
        )
    except (PlaywrightTimeout, TimeoutError) as exc:
        if main:
            raise CaptureError("timed out") from exc
        route.abort()
        return
    except (Error, OSError) as exc:
        if main:
            raise CaptureError(str(exc) or "fetch failed") from exc
        route.abort()
        return
    locations = _locations(response)
    if 300 <= response.status < 400:
        if len(locations) > 1:
            if main:
                raise FetchError("multiple location headers")
            route.abort()
            return
        if locations:
            try:
                require_public_http(urljoin(url, locations[0]))
            except FetchError:
                if main:
                    raise
                route.abort()
                return
    if len(response.body()) > MAX_HOP_BYTES:
        if main:
            raise CaptureError("archive too large")
        route.abort()
        return
    route.fulfill(response=response)


def _is_main_navigation(request: Request) -> bool:
    if not request.is_navigation_request():
        return False
    return request.frame.parent_frame is None


def _is_public_http(url: str) -> bool:
    try:
        require_public_http(url)
        return True
    except FetchError:
        return False
