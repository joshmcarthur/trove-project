from django.db.utils import OperationalError, ProgrammingError
from django.urls import path

from trove.services.clients import ensure_wall_client

from .items import items
from .page import WallPage, vendor


def setup(core, entry):
    try:
        ensure_wall_client()
    except (OperationalError, ProgrammingError):
        pass
    core.add_urls(
        [
            path("api/items", items, name="items"),
            path("vendor/<str:name>", vendor, name="vendor"),
            path("", WallPage.as_view(), name="page"),
        ]
    )
