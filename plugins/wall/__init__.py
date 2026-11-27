from django.db.utils import OperationalError, ProgrammingError
from django.http import JsonResponse
from django.urls import path

from trove.services.clients import ensure_wall_client
from trove.services.tokens import agent_for_request

from .queries import wall_items

READ_SCOPE = "read:all"


def items(request):
    if agent_for_request(request, READ_SCOPE) is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    return JsonResponse(wall_items())


def setup(core, entry):
    try:
        ensure_wall_client()
    except (OperationalError, ProgrammingError):
        pass
    core.add_urls([path("api/items", items, name="items")])
