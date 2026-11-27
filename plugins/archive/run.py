from __future__ import annotations

import ipaddress
import socket
import urllib.request
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request

from trove.tasks import Emit, Result

TIMEOUT = 30
MAX_BYTES = 20 * 1024 * 1024
CONTENT_TYPE = "multipart/related"
ARCHIVE_KEY = "mhtml"
USER_AGENT = "Trove/0.1 (archive)"


class ArchiveError(Exception):
    pass


class _PublicRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        _require_public_http(newurl)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def run(item, ctx):
    url = ctx.values(item).get("url")
    if not isinstance(url, str) or not url.strip():
        raise ArchiveError("link has no url")
    data = _fetch(url.strip())
    return Result(
        emits=[
            Emit(
                type="capture.file",
                key=ARCHIVE_KEY,
                blob=data,
                content_type=CONTENT_TYPE,
            )
        ],
    )


def _build_opener():
    return urllib.request.build_opener(_PublicRedirectHandler)


def _open(request, timeout):
    return _build_opener().open(request, timeout=timeout)


def _fetch(url: str) -> bytes:
    _require_public_http(url)
    request = Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with _open(request, timeout=TIMEOUT) as response:
            _require_public_http(response.geturl())
            data = response.read(MAX_BYTES + 1)
    except HTTPError as exc:
        raise ArchiveError(f"HTTP {exc.code}") from exc
    except (TimeoutError, URLError, OSError) as exc:
        raise ArchiveError(str(exc) or "fetch failed") from exc
    if len(data) > MAX_BYTES:
        raise ArchiveError("archive too large")
    return data


def _require_public_http(url: str):
    parsed = urlparse(url)
    if parsed.scheme.lower() not in {"http", "https"}:
        raise ArchiveError("only http(s) urls")
    host = parsed.hostname
    if not host:
        raise ArchiveError("url has no host")
    for ip in _host_ips(host):
        if not _is_public_ip(ip):
            raise ArchiveError("url is not public")


def _host_ips(host: str) -> list[ipaddress.IPv4Address | ipaddress.IPv6Address]:
    try:
        return [_unwrap(ipaddress.ip_address(host))]
    except ValueError:
        pass
    if _is_localhost_name(host):
        return [ipaddress.ip_address("127.0.0.1")]
    try:
        infos = socket.getaddrinfo(host, None)
    except OSError as exc:
        raise ArchiveError("host lookup failed") from exc
    ips: list[ipaddress.IPv4Address | ipaddress.IPv6Address] = []
    seen: set[ipaddress.IPv4Address | ipaddress.IPv6Address] = set()
    for info in infos:
        ip = _unwrap(ipaddress.ip_address(info[4][0]))
        if ip in seen:
            continue
        seen.add(ip)
        ips.append(ip)
    if not ips:
        raise ArchiveError("host has no addresses")
    return ips


def _is_localhost_name(host: str) -> bool:
    name = host.lower().rstrip(".")
    return name == "localhost" or name.endswith(".localhost")


def _unwrap(
    ip: ipaddress.IPv4Address | ipaddress.IPv6Address,
) -> ipaddress.IPv4Address | ipaddress.IPv6Address:
    mapped = getattr(ip, "ipv4_mapped", None)
    return mapped if mapped is not None else ip


def _is_public_ip(ip: ipaddress.IPv4Address | ipaddress.IPv6Address) -> bool:
    return ip.is_global and not ip.is_multicast
