"""Fetch bytes from a public http(s) URL without file or ftp handlers."""

import urllib.request
from urllib.error import HTTPError, URLError
from urllib.request import Request

from trove.services.fetch_error import FetchError
from trove.services.public_redirect_handler import PublicRedirectHandler
from trove.services.public_url import require_public_http

TIMEOUT = 10


def make_opener(*extra_handlers):
    opener = urllib.request.OpenerDirector()
    for handler in (
        *extra_handlers,
        urllib.request.UnknownHandler(),
        urllib.request.HTTPHandler(),
        urllib.request.HTTPSHandler(),
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
    require_public_http(url)
    request = Request(url, headers={"User-Agent": user_agent})
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
