from __future__ import annotations

from trove.services.fetch_error import FetchError
from trove.services.public_url import require_public_http
from trove.tasks import Emit, Result

from .chromium import CaptureError, capture_mhtml

TIMEOUT = 30
MAX_BYTES = 20 * 1024 * 1024
CONTENT_TYPE = "multipart/related"
ARCHIVE_KEY = "mhtml"


class ArchiveError(Exception):
    pass


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


def _fetch(url: str) -> bytes:
    try:
        require_public_http(url)
        data = capture_mhtml(url, timeout=TIMEOUT)
    except FetchError as exc:
        raise ArchiveError(str(exc)) from exc
    except CaptureError as exc:
        raise ArchiveError(str(exc)) from exc
    if len(data) > MAX_BYTES:
        raise ArchiveError("archive too large")
    return data
