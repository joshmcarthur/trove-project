from django.http import JsonResponse

from trove.services.tokens import agent_for_request

from .queries import wall_items

READ_SCOPE = "read:all"


def items(request):
    if agent_for_request(request, READ_SCOPE) is None:
        return JsonResponse({"error": "unauthorised"}, status=401)
    return JsonResponse(wall_items())
