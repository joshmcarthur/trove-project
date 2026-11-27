import re

from django.http import FileResponse, Http404, JsonResponse

from trove.models import Blob
from trove.services.blobs import blob_path
from trove.services.tokens import agent_for_request

READ_SCOPE = "read:all"
SHA256 = re.compile(r"^[0-9a-f]{64}$")


def blob(request, sha):
    if agent_for_request(request, READ_SCOPE) is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    if SHA256.fullmatch(sha) is None:
        raise Http404()
    row = Blob.objects.filter(sha256=sha).first()
    path = blob_path(sha)
    if row is None or not path.is_file():
        raise Http404()
    try:
        handle = path.open("rb")
    except OSError as exc:
        raise Http404() from exc
    response = FileResponse(handle, content_type=row.content_type)
    response["Cache-Control"] = "private, no-store"
    return response
