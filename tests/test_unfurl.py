from urllib.parse import urlparse

import pytest

from tests.factories import (
    Fetched,
    ItemFactory,
    http_404,
    png_bytes,
    stub_public_http,
    timed_out,
)
from trove.core import get_core, reset_core
from trove.models import Activity, Claim, Item, Link
from trove.services.recorder import record
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])

PAGE = "https://example.com/lamp"
YOUTUBE = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
HTML = """<!doctype html>
<html><head>
<title>Fallback title</title>
<meta property="og:title" content="Hallway lamp">
<meta property="og:image" content="https://cdn.example.com/lamp.png">
</head></html>
"""


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def make_link(url, title=None):
    with record("ingest", "test") as rec:
        item = rec.item("capture.link")
        rec.claim(item, "url", url, rank=Claim.Rank.PREFERRED)
        if title:
            rec.claim(item, "title", title, rank=Claim.Rank.PREFERRED)
    return item


def pages(html=HTML, image=None, image_type="image/png"):
    thumb = image if image is not None else png_bytes()

    def urlopen(request, timeout=None):
        url = request.full_url
        path = urlparse(url).path
        if path.endswith((".png", ".jpg", ".jpeg", ".webp")):
            return Fetched(thumb, image_type, url)
        return Fetched(html, "text/html; charset=utf-8", url)

    return urlopen


def pdf_body(request, timeout=None):
    return Fetched(b"%PDF", "application/pdf", request.full_url)


def unfurl(core, item):
    execute_derive(core.processors["unfurl"], item.id)
    item.refresh_from_db()
    return item


def test_unfurl_claims_title_and_preview(core, blob_dir, monkeypatch):
    stub_public_http(monkeypatch, pages())
    item = unfurl(core, make_link(YOUTUBE))
    assert item.current_values()["title"] == "Hallway lamp"
    assert Claim.objects.get(item=item, path="title").rank == Claim.Rank.NORMAL
    preview = Item.objects.get(type="capture.image")
    values = preview.current_values()
    assert values["$blob"]
    assert values["title"] == "Hallway lamp"
    assert (blob_dir / values["$blob"][:2]).exists()
    assert (
        Link.objects.live()
        .filter(source=preview, target=item, rel="derived_from")
        .exists()
    )
    act = Activity.objects.get(agent="unfurl@1", subject=item)
    assert act.outcome == "ok"


def test_typed_title_beats_fetched_title(core, blob_dir, monkeypatch):
    stub_public_http(monkeypatch, pages())
    item = unfurl(core, make_link(PAGE, title="mine"))
    assert item.current_values()["title"] == "mine"
    ranks = list(
        Claim.objects.filter(item=item, path="title").values_list("rank", flat=True)
    )
    assert Claim.Rank.PREFERRED in ranks
    assert Claim.Rank.NORMAL in ranks


def test_missing_url_fails_and_keeps_the_item(core):
    item = ItemFactory(type="capture.link")
    unfurl(core, item)
    act = Activity.objects.get(agent="unfurl@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id).exists()


def test_file_url_fails_without_fetching(core, monkeypatch):
    seen = []

    def urlopen(request, timeout=None):
        seen.append(request.full_url)
        return Fetched(HTML)

    stub_public_http(monkeypatch, urlopen)
    item = unfurl(core, make_link("file:///etc/passwd"))
    assert seen == []
    act = Activity.objects.get(agent="unfurl@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()


@pytest.mark.parametrize("fake", [http_404, timed_out, pdf_body])
def test_fetch_failures_fail_derive_and_keep_the_link(core, monkeypatch, fake):
    stub_public_http(monkeypatch, fake)
    item = unfurl(core, make_link(PAGE))
    act = Activity.objects.get(agent="unfurl@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.image").exists()
    assert "title" not in item.current_values()
