"""Core plugin: accept a POSTed file plus metadata from any authenticated client."""

import io

from django.conf import settings
from django.http import JsonResponse
from django.urls import path
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_POST
from pydantic import BaseModel

from trove.models import Claim
from trove.services.recorder import record

PREFERRED = Claim.Rank.PREFERRED


class Capture(BaseModel):
    title: str | None = None
    notes: str | None = None
    url: str | None = None


def _client(request):
    """Spike stand-in for django-oauth-toolkit: bearer token maps to a client name."""
    auth = request.headers.get("Authorization", "")
    token = auth.removeprefix("Bearer ").strip()
    return settings.TROVE["INGEST_TOKENS"].get(token)


def _file_metadata(data: bytes, content_type: str) -> dict:
    if not content_type.startswith("image/"):
        return {}
    try:
        from PIL import Image

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
            item = rec.item(
                "capture.image" if ctype.startswith("image/") else "capture.file"
            )
            rec.claim(item, "$blob", rec.blob(data, ctype))
            for key, value in _file_metadata(data, ctype).items():
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
