"""Fetch bytes from a public http(s) URL without file or ftp handlers."""

from __future__ import annotations

import errno
import http.client
import socket
import sys
import urllib.request
from urllib.error import HTTPError, URLError
from urllib.request import Request

from trove.services.fetch_error import FetchError
from trove.services.public_redirect_handler import PublicRedirectHandler
from trove.services.public_url import require_public_http

TIMEOUT = 10


def _connect_pinned(conn) -> None:
    sys.audit("http.client.connect", conn, conn.host, conn.port)
    conn.sock = socket.create_connection(
        (conn._pinned_ip, conn.port),
        conn.timeout,
        conn.source_address,
    )
    try:
        conn.sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
    except OSError as exc:
        if exc.errno != errno.ENOPROTOOPT:
            raise
    if conn._tunnel_host:
        conn._tunnel()


class PinnedHTTPConnection(http.client.HTTPConnection):
    def __init__(self, host, port=None, *, pinned_ip, **kwargs):
        super().__init__(host, port, **kwargs)
        self._pinned_ip = pinned_ip

    def connect(self):
        _connect_pinned(self)


class PinnedHTTPSConnection(http.client.HTTPSConnection):
    def __init__(self, host, port=None, *, pinned_ip, **kwargs):
        super().__init__(host, port, **kwargs)
        self._pinned_ip = pinned_ip

    def connect(self):
        _connect_pinned(self)
        if self._tunnel_host:
            server_hostname = self._tunnel_host
        else:
            server_hostname = self.host
        self.sock = self._context.wrap_socket(
            self.sock, server_hostname=server_hostname
        )


class PinnedHTTPHandler(urllib.request.HTTPHandler):
    def http_open(self, req):
        pinned_ip = _pinned_ip(req)

        def connection(host, **kwargs):
            return PinnedHTTPConnection(host, pinned_ip=pinned_ip, **kwargs)

        return self.do_open(connection, req)


class PinnedHTTPSHandler(urllib.request.HTTPSHandler):
    def https_open(self, req):
        pinned_ip = _pinned_ip(req)

        def connection(host, **kwargs):
            return PinnedHTTPSConnection(host, pinned_ip=pinned_ip, **kwargs)

        return self.do_open(connection, req)


def _pinned_ip(req) -> str:
    pinned = getattr(req, "pinned_ip", None)
    if pinned:
        return pinned
    return require_public_http(req.full_url)


def make_opener(*extra_handlers):
    opener = urllib.request.OpenerDirector()
    for handler in (
        *extra_handlers,
        urllib.request.UnknownHandler(),
        PinnedHTTPHandler(),
        PinnedHTTPSHandler(),
        urllib.request.HTTPDefaultErrorHandler(),
        urllib.request.HTTPErrorProcessor(),
        PublicRedirectHandler(),
    ):
        opener.add_handler(handler)
    return opener


_OPENER = make_opener()


def urlopen(request, timeout=None):
    return _OPENER.open(request, timeout=timeout)


def fetch_public_http(
    url: str, *, max_bytes: int, user_agent: str
) -> tuple[bytes, str, str, str]:
    ip = require_public_http(url)
    request = Request(url, headers={"User-Agent": user_agent})
    request.pinned_ip = ip
    try:
        with urlopen(request, timeout=TIMEOUT) as response:
            require_public_http(response.geturl())
            content_type = response.headers.get_content_type()
            charset = response.headers.get_content_charset() or "utf-8"
            data = response.read(max_bytes + 1)
            final_url = response.geturl()
    except FetchError:
        raise
    except HTTPError as exc:
        raise FetchError(f"HTTP {exc.code}") from exc
    except (TimeoutError, URLError, OSError) as exc:
        raise FetchError(str(exc) or "fetch failed") from exc
    return data, content_type, final_url, charset
