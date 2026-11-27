"""Core plugin: accept a POSTed file plus metadata from any authenticated client."""

import io
import re
from urllib.parse import urlparse

from django.conf import settings
from django.http import JsonResponse
from django.urls import path
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_POST
from PIL import Image
from pydantic import BaseModel

from trove.models import Claim
from trove.services.recorder import record
from trove.services.tokens import INGEST_SCOPE, agent_for_request

PREFERRED = Claim.Rank.PREFERRED
_PDF_COUNT = re.compile(rb"/Count\s+(\d+)")
_PDF_PAGE = re.compile(rb"/Type\s*/Page(?!s)")
_VIDEO_BRANDS = {
    b"av01",
    b"avc1",
    b"iso2",
    b"isom",
    b"mp41",
    b"mp42",
    b"M4V ",
    b"qt  ",
}
_HEIF_BRANDS = {b"heic", b"heix", b"hevc", b"mif1", b"msf1"}
_CONTAINER_BOXES = {b"mdia", b"minf", b"moov", b"stbl", b"trak"}
_VISUAL_SAMPLE = {b"av01", b"avc1", b"avc3", b"hev1", b"hvc1", b"mp4v", b"vp09"}


class Capture(BaseModel):
    title: str | None = None
    notes: str | None = None
    url: str | None = None


def _client(request):
    return agent_for_request(request, INGEST_SCOPE)


def _file_metadata(data: bytes) -> dict:
    try:
        with Image.open(io.BytesIO(data)) as img:
            return {"width": img.width, "height": img.height}
    except Exception:  # noqa: BLE001
        return {}


def _is_pdf(data: bytes, ctype: str) -> bool:
    return data.startswith(b"%PDF") or ctype.startswith("application/pdf")


def _pdf_metadata(data: bytes) -> dict:
    trailer = data.rfind(b"trailer")
    if trailer != -1 and b"/Encrypt" in data[trailer:]:
        return {}
    try:
        counts = [int(n) for n in _PDF_COUNT.findall(data)]
        if counts:
            return {"pages": max(counts)}
        pages = len(_PDF_PAGE.findall(data))
        return {"pages": pages} if pages else {}
    except Exception:  # noqa: BLE001
        return {}


def _ftyp_brands(data: bytes) -> set[bytes]:
    if len(data) < 16 or data[4:8] != b"ftyp":
        return set()
    size = int.from_bytes(data[:4], "big")
    if size < 16 or size > len(data):
        return set()
    payload = data[8:size]
    brands = {payload[:4]}
    for i in range(8, len(payload) - 3, 4):
        brands.add(payload[i : i + 4])
    return brands


def _is_video(data: bytes, ctype: str) -> bool:
    if ctype.startswith("video/"):
        return True
    brands = _ftyp_brands(data)
    if brands & _HEIF_BRANDS:
        return False
    return bool(brands & _VIDEO_BRANDS)


def _iter_boxes(data: bytes, start: int = 0, end: int | None = None):
    if end is None:
        end = len(data)
    pos = start
    while pos + 8 <= end:
        size = int.from_bytes(data[pos : pos + 4], "big")
        typ = data[pos + 4 : pos + 8]
        header = 8
        if size == 1:
            if pos + 16 > end:
                break
            size = int.from_bytes(data[pos + 8 : pos + 16], "big")
            header = 16
        elif size == 0:
            size = end - pos
        if size < header or pos + size > end:
            break
        yield typ, pos + header, pos + size
        pos += size


def _seconds(timescale: int, duration: int):
    if not timescale:
        return None
    seconds = duration / timescale
    return int(seconds) if seconds.is_integer() else seconds


def _read_mvhd(data: bytes, start: int, end: int, meta: dict):
    if end - start < 20:
        return
    version = data[start]
    if version == 1:
        if end - start < 32:
            return
        timescale = int.from_bytes(data[start + 20 : start + 24], "big")
        duration = int.from_bytes(data[start + 24 : start + 32], "big")
    else:
        timescale = int.from_bytes(data[start + 12 : start + 16], "big")
        duration = int.from_bytes(data[start + 16 : start + 20], "big")
    value = _seconds(timescale, duration)
    if value is not None:
        meta["duration"] = value


def _read_tkhd(data: bytes, start: int, end: int, meta: dict):
    version = data[start] if start < end else 0
    width_at = 88 if version == 1 else 76
    if end - start < width_at + 8:
        return
    width = int.from_bytes(data[start + width_at : start + width_at + 4], "big") >> 16
    height = (
        int.from_bytes(data[start + width_at + 4 : start + width_at + 8], "big") >> 16
    )
    if width and height:
        meta.setdefault("width", width)
        meta.setdefault("height", height)


def _read_stsd(data: bytes, start: int, end: int, meta: dict):
    if end - start < 8:
        return
    count = int.from_bytes(data[start + 4 : start + 8], "big")
    pos = start + 8
    for _ in range(count):
        if pos + 8 > end:
            return
        size = int.from_bytes(data[pos : pos + 4], "big")
        typ = data[pos + 4 : pos + 8]
        if typ in _VISUAL_SAMPLE and size >= 36 and pos + 36 <= end:
            width = int.from_bytes(data[pos + 32 : pos + 34], "big")
            height = int.from_bytes(data[pos + 34 : pos + 36], "big")
            if width and height:
                meta["width"] = width
                meta["height"] = height
                return
        pos += size if size >= 8 else 8


def _scan_video(data: bytes, start: int, end: int, meta: dict):
    for typ, payload_start, payload_end in _iter_boxes(data, start, end):
        if typ in _CONTAINER_BOXES:
            _scan_video(data, payload_start, payload_end, meta)
        elif typ == b"mvhd":
            _read_mvhd(data, payload_start, payload_end, meta)
        elif typ == b"tkhd":
            _read_tkhd(data, payload_start, payload_end, meta)
        elif typ == b"stsd":
            _read_stsd(data, payload_start, payload_end, meta)


def _video_metadata(data: bytes) -> dict:
    meta: dict = {}
    try:
        _scan_video(data, 0, len(data), meta)
    except Exception:  # noqa: BLE001
        return {}
    return meta


def _upload_type_and_meta(data: bytes, ctype: str) -> tuple[str, dict]:
    if _is_pdf(data, ctype):
        return "capture.document", _pdf_metadata(data)
    if _is_video(data, ctype):
        return "capture.video", _video_metadata(data)
    meta = _file_metadata(data)
    if ctype.startswith("image/") or meta:
        return "capture.image", meta
    return "capture.file", {}


def _posted_url(request):
    raw = request.POST.get("url")
    if raw is None:
        return None
    url = raw.strip()
    return url or None


def _content_length(request):
    raw = request.META.get("CONTENT_LENGTH")
    if not raw:
        return None
    try:
        return int(raw)
    except ValueError:
        return None


def _too_large(size):
    return size > settings.TROVE["INGEST_MAX_BYTES"]


def _reject_too_large():
    return JsonResponse({"error": "file too large"}, status=413)


@csrf_exempt
@require_POST
def ingest(request):
    agent = _client(request)
    if agent is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    length = _content_length(request)
    if length is not None and _too_large(length):
        return _reject_too_large()
    upload = request.FILES.get("file")
    url = _posted_url(request)
    if url is not None and urlparse(url).scheme == "data":
        return JsonResponse({"error": "data urls are not allowed"}, status=400)
    data, ctype = None, None
    if upload is not None:
        if _too_large(upload.size):
            return _reject_too_large()
        data = upload.read()
        ctype = upload.content_type or "application/octet-stream"
        if not data:
            return JsonResponse({"error": "empty file"}, status=400)
    elif not url:
        return JsonResponse({"error": "send a file or a url"}, status=400)
    with record("ingest", agent) as rec:
        if data is not None:
            item_type, meta = _upload_type_and_meta(data, ctype)
            item = rec.item(item_type)
            rec.claim(item, "$blob", rec.blob(data, ctype))
            for key, value in meta.items():
                rec.claim(item, key, value)
        else:
            item = rec.item("capture.link")
        if url:
            rec.claim(item, "url", url, rank=PREFERRED)
        for key in ("title", "notes"):
            if request.POST.get(key):
                rec.claim(item, key, request.POST[key], rank=PREFERRED)
    return JsonResponse({"id": item.id, "type": item.type}, status=201)


def setup(core, entry):
    for name in (
        "capture.image",
        "capture.file",
        "capture.link",
        "capture.document",
        "capture.video",
    ):
        core.add_type(name, Capture)
    core.add_urls([path("", ingest, name="ingest")])
