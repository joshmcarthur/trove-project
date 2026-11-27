from pathlib import Path

from django.http import Http404, HttpResponse
from django.template import Context, Template
from django.urls import reverse
from django.views import View

from trove.services.clients import ensure_wall_client

PLUGIN_DIR = Path(__file__).parent
STATIC_DIR = PLUGIN_DIR / "static"
VENDOR_TYPES = {
    "preact.module.js": "text/javascript; charset=utf-8",
    "htm.module.js": "text/javascript; charset=utf-8",
    "wall.js": "text/javascript; charset=utf-8",
    "wall.css": "text/css; charset=utf-8",
}


class WallPage(View):
    def get(self, request):
        app = ensure_wall_client()
        html = Template((PLUGIN_DIR / "page.html").read_text()).render(
            Context(
                {
                    "client_id": app.client_id,
                    "authorize_url": reverse("oauth2_provider:authorize"),
                    "token_url": reverse("oauth2_provider:token"),
                }
            )
        )
        return HttpResponse(html)


def vendor(request, name):
    content_type = VENDOR_TYPES.get(name)
    if content_type is None:
        raise Http404()
    path = (STATIC_DIR / name).resolve()
    if path.parent != STATIC_DIR.resolve() or not path.is_file():
        raise Http404()
    return HttpResponse(path.read_bytes(), content_type=content_type)
