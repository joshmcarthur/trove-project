"""Refuse http(s) URLs whose host is private, loopback, or otherwise blocked."""

import ipaddress
from socket import getaddrinfo
from urllib.parse import urlparse

from trove.services.fetch_error import FetchError

_BLOCKED_HOSTS = {"localhost", "metadata.google.internal"}
_BLOCKED_SUFFIXES = (".local", ".ts.net", ".internal")
_CGNAT = ipaddress.ip_network("100.64.0.0/10")


def require_public_http(url: str) -> None:
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
        infos = getaddrinfo(host, None)
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
