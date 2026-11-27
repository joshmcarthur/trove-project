"""Fetch public PDF or image bytes during URL ingest."""

import ipaddress
import socket
import urllib.request
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request

TIMEOUT = 10
USER_AGENT = "Trove/0.1 (ingest)"
FILE_SUFFIXES = (".pdf", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif")
_BLOCKED_HOSTS = {"localhost", "metadata.google.internal"}
_BLOCKED_SUFFIXES = (".local", ".ts.net", ".internal")
_CGNAT = ipaddress.ip_network("100.64.0.0/10")


class FetchError(Exception):
    pass


def looks_like_file_url(url: str) -> bool:
    return urlparse(url).path.lower().endswith(FILE_SUFFIXES)


def fetch_public_file(url: str, max_bytes: int) -> tuple[bytes, str]:
    _require_public_http(url)
    request = Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with urllib.request.urlopen(request, timeout=TIMEOUT) as response:
            _require_public_http(response.geturl())
            content_type = response.headers.get_content_type()
            length = response.headers.get("Content-Length")
            if length:
                try:
                    if int(length) > max_bytes:
                        raise FetchError("file too large")
                except ValueError:
                    pass
            data = response.read(max_bytes + 1)
    except FetchError:
        raise
    except HTTPError as exc:
        raise FetchError(f"HTTP {exc.code}") from exc
    except (TimeoutError, URLError, OSError) as exc:
        raise FetchError(str(exc) or "fetch failed") from exc
    if not data:
        raise FetchError("empty file")
    if len(data) > max_bytes:
        raise FetchError("file too large")
    return data, content_type


def _require_public_http(url: str):
    parsed = urlparse(url)
    if parsed.scheme.lower() not in {"http", "https"}:
        raise FetchError("only http(s) urls")
    host = parsed.hostname
    if not host:
        raise FetchError("url has no host")
    lowered = host.lower().rstrip(".")
    if lowered in _BLOCKED_HOSTS or lowered.endswith(_BLOCKED_SUFFIXES):
        raise FetchError("private url")
    for ip in _host_ips(lowered):
        if _is_private(ip):
            raise FetchError("private url")


def _host_ips(host: str):
    try:
        return [ipaddress.ip_address(host)]
    except ValueError:
        pass
    try:
        infos = socket.getaddrinfo(host, None)
    except OSError as exc:
        raise FetchError("dns failed") from exc
    ips = []
    for info in infos:
        addr = info[4][0]
        if "%" in addr:
            addr = addr.split("%", 1)[0]
        ips.append(ipaddress.ip_address(addr))
    if not ips:
        raise FetchError("dns failed")
    return ips


def _is_private(ip) -> bool:
    if (
        ip.is_private
        or ip.is_loopback
        or ip.is_link_local
        or ip.is_multicast
        or ip.is_unspecified
    ):
        return True
    if ip.version == 4 and ip in _CGNAT:
        return True
    mapped = getattr(ip, "ipv4_mapped", None)
    return mapped is not None and _is_private(mapped)
