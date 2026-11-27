"""Core plugin: accept a POSTed file plus metadata from any authenticated client."""

import io

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
HEIC_BRANDS = {b"heic", b"heif", b"mif1", b"msf1", b"heix", b"heim", b"heis"}


class Capture(BaseModel):
    title: str | None = None
    notes: str | None = None
    url: str | None = None


def _client(request):
    return agent_for_request(request, INGEST_SCOPE)


def _looks_like_heic(data: bytes, name: str = "") -> bool:
    if name.lower().endswith((".heic", ".heif")):
        return True
    brand = data[8:12] if len(data) >= 12 and data[4:8] == b"ftyp" else b""
    return brand in HEIC_BRANDS


def _is_image(data: bytes, content_type: str, name: str = "") -> bool:
    return content_type.startswith("image/") or _looks_like_heic(data, name)


def _file_metadata(data: bytes, content_type: str, name: str = "") -> dict:
    if not _is_image(data, content_type, name):
        return {}
    try:
        with Image.open(io.BytesIO(data)) as img:
            return {"width": img.width, "height": img.height}
    except Exception:  # noqa: BLE001
        return {}


@csrf_exempt
@require_POST
def ingest(request):
    agent = _client(request)
    if agent is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    upload = request.FILES.get("file")
    url = request.POST.get("url")
    if upload is None and not url:
        return JsonResponse({"error": "send a file or a url"}, status=400)
    with record("ingest", agent) as rec:
        if upload is not None:
            data, ctype = (
                upload.read(),
                upload.content_type or "application/octet-stream",
            )
            name = getattr(upload, "name", "") or ""
            item = rec.item(
                "capture.image" if _is_image(data, ctype, name) else "capture.file"
            )
            rec.claim(item, "$blob", rec.blob(data, ctype))
            for key, value in _file_metadata(data, ctype, name).items():
                rec.claim(item, key, value)
        else:
            item = rec.item("capture.link")
        for key in ("title", "notes", "url"):
            if request.POST.get(key):
                rec.claim(item, key, request.POST[key], rank=PREFERRED)
    return JsonResponse({"id": item.id, "type": item.type}, status=201)


def setup(core, entry):
    for name in ("capture.image", "capture.file", "capture.link"):
        core.add_type(name, Capture)
    core.add_urls([path("", ingest, name="ingest")])
