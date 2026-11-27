from django.contrib import admin
from django.urls import include, path

from trove.runtime.core import get_core


def _plugin_urlpatterns():
    core = get_core()
    patterns = []
    for domain, plugin in core.plugins.items():
        if plugin.status == "loaded" and plugin.urls:
            patterns.append(path(f"{domain}/", include((plugin.urls, domain))))
    return patterns


urlpatterns = [
    path("admin/", admin.site.urls),
    *_plugin_urlpatterns(),
]
