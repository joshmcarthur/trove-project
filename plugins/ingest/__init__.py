"""Core plugin: accept a POSTed file plus metadata from any authenticated client."""

import io
import re
from urllib.parse import urlparse

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


def _upload_type_and_meta(data: bytes, ctype: str) -> tuple[str, dict]:
    if _is_pdf(data, ctype):
        return "capture.document", _pdf_metadata(data)
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


@csrf_exempt
@require_POST
def ingest(request):
    agent = _client(request)
    if agent is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    upload = request.FILES.get("file")
    url = _posted_url(request)
    if url is not None and urlparse(url).scheme == "data":
        return JsonResponse({"error": "data urls are not allowed"}, status=400)
    data, ctype = None, None
    if upload is not None:
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
    ):
        core.add_type(name, Capture)
    core.add_urls([path("", ingest, name="ingest")])
