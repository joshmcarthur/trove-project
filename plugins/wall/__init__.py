from django.http import JsonResponse
from django.urls import path

from .queries import wall_items


def items(request):
    return JsonResponse(wall_items())


def setup(core, entry):
    core.add_urls([path("api/items", items, name="items")])
