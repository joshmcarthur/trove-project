"""Load a public URL in bundled Chromium and snapshot it as MHTML."""

from __future__ import annotations

from contextlib import contextmanager
from urllib.parse import urlparse

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


class CaptureError(Exception):
    pass


@contextmanager
def launch_chromium():
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        try:
            yield browser
        finally:
            browser.close()


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


def _snapshot(browser, url: str, *, timeout_ms: int) -> bytes:
    context = browser.new_context(accept_downloads=False)
    blocked_nav: list[str] = []
    try:
        page = context.new_page()
        context.set_default_timeout(timeout_ms)
        page.on("download", lambda download: download.cancel())
        page.route("**/*", lambda route: handle_route(route, blocked_nav))
        try:
            page.goto(url, timeout=timeout_ms, wait_until="load")
        except Error:
            if blocked_nav:
                raise FetchError("private url") from None
            raise
        if blocked_nav:
            raise FetchError("private url")
        require_public_http(page.url)
        data = _cdp_mhtml(page)
    finally:
        context.close()
    if isinstance(data, bytes):
        return data
    return data.encode()


def _cdp_mhtml(page) -> str:
    session = page.context.new_cdp_session(page)
    result = session.send(SNAPSHOT_METHOD, SNAPSHOT_PARAMS)
    data = result.get("data")
    if not data:
        raise CaptureError("empty snapshot")
    return data


def handle_route(route, blocked_nav: list[str]) -> None:
    request = route.request
    url = request.url
    if _is_main_navigation(request):
        if _is_public_http(url):
            route.continue_()
        else:
            blocked_nav.append(url)
            route.abort()
        return
    if _subresource_allowed(url):
        route.continue_()
    else:
        route.abort()


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
