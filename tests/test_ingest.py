import io
from email.message import EmailMessage
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, urlparse

import pytest
from django.core.files.uploadedfile import SimpleUploadedFile
from django.test import Client
from PIL import Image

from tests.factories import seed_dev_ingest_token
from trove.models import Activity, Blob, Claim, Item
from trove.services.owner import ensure_owner

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}
FIXTURES = Path(__file__).parent / "fixtures"
TINY_PDF = FIXTURES / "tiny.pdf"
ENCRYPTED_PDF = FIXTURES / "encrypted.pdf"
TINY_MP4 = FIXTURES / "tiny.mp4"
TINY_HEIC = FIXTURES / "tiny.heic"

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture(autouse=True)
def ingest_token():
    seed_dev_ingest_token()


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def pdf(path=TINY_PDF, name="spec.pdf", content_type="application/pdf"):
    return SimpleUploadedFile(name, path.read_bytes(), content_type)


def video(path=TINY_MP4, name="lamp.mp4", content_type="video/mp4"):
    return SimpleUploadedFile(name, path.read_bytes(), content_type)


def audio_only_mp4():
    def u32(n):
        return n.to_bytes(4, "big")

    def box(typ, payload):
        return u32(8 + len(payload)) + typ + payload

    identity = (
        u32(0x00010000) + u32(0) * 3 + u32(0x00010000) + u32(0) * 3 + u32(0x40000000)
    )
    mvhd = box(
        b"mvhd",
        b"\x00" * 12
        + u32(1000)
        + u32(500)
        + u32(0x00010000)
        + b"\x01\x00\x00\x00"
        + u32(0) * 2
        + identity
        + b"\x00" * 24
        + u32(2),
    )
    return box(b"ftyp", b"isom" + u32(0) + b"isommp41") + box(b"moov", mvhd)


def test_url_only_creates_link_with_preferred_claims(client):
    response = client.post(
        "/ingest/",
        {
            "url": "https://example.com/lamp",
            "title": "hallway",
            "notes": "warm",
        },
        **TOKEN,
    )
    assert response.status_code == 201
    payload = response.json()
    assert payload["type"] == "capture.link"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert values["url"] == "https://example.com/lamp"
    assert values["title"] == "hallway"
    assert values["notes"] == "warm"
    ranks = {
        claim.path: claim.rank
        for claim in Claim.objects.filter(item=item, path__in=("url", "title", "notes"))
    }
    assert ranks == {
        "url": Claim.Rank.PREFERRED,
        "title": Claim.Rank.PREFERRED,
        "notes": Claim.Rank.PREFERRED,
    }


@pytest.mark.parametrize(
    "url",
    [
        "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        "https://vimeo.com/123456789",
        "https://x.com/trove/status/1",
        "https://example.com/blog/post",
    ],
)
def test_page_urls_take_the_link_path(client, url):
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 201
    assert response.json()["type"] == "capture.link"
    item = Item.objects.get(id=response.json()["id"])
    assert item.current_values()["url"] == url


def test_missing_url_and_file_is_400(client):
    response = client.post("/ingest/", {}, **TOKEN)
    assert response.status_code == 400


def test_unauthenticated_url_is_401(client):
    response = client.post("/ingest/", {"url": "https://example.com"})
    assert response.status_code == 401


@pytest.mark.parametrize("url", ["", "   ", "data:text/plain,hello"])
def test_empty_or_data_url_is_400(client, url):
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 400
    assert not Item.objects.exists()


def test_same_url_twice_is_two_items(client):
    url = "https://example.com/again"
    first = client.post("/ingest/", {"url": url}, **TOKEN)
    second = client.post("/ingest/", {"url": url}, **TOKEN)
    assert first.status_code == 201
    assert second.status_code == 201
    assert first.json()["id"] != second.json()["id"]
    assert Item.objects.filter(type="capture.link").count() == 2


def test_pdf_becomes_document_with_page_count(client, blob_dir):
    response = client.post("/ingest/", {"file": pdf(), "title": "spec"}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.document"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert values["pages"] == 1
    assert values["title"] == "spec"
    assert (blob_dir / values["$blob"][:2]).exists()
    assert Claim.objects.get(item=item, path="pages").rank == Claim.Rank.NORMAL


def test_pdf_octet_stream_is_still_a_document(client, blob_dir):
    response = client.post(
        "/ingest/",
        {"file": pdf(content_type="application/octet-stream")},
        **TOKEN,
    )
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.document"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert values["pages"] == 1
    assert "$blob" in values


def test_junk_bytes_are_a_file_blob_without_pages(client, blob_dir):
    junk = SimpleUploadedFile(
        "notes.bin", b"\x00\xff not a document", "application/octet-stream"
    )
    response = client.post("/ingest/", {"file": junk}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.file"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert "$blob" in values
    assert "pages" not in values
    assert (blob_dir / values["$blob"][:2]).exists()


def test_docx_is_a_file_blob_without_pretending_to_parse(client, blob_dir):
    docx = SimpleUploadedFile(
        "manual.docx",
        b"PK\x03\x04junk",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    )
    response = client.post("/ingest/", {"file": docx}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.file"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert "$blob" in values
    assert "pages" not in values


def test_empty_file_is_400(client):
    empty = SimpleUploadedFile("empty.pdf", b"", "application/pdf")
    response = client.post("/ingest/", {"file": empty}, **TOKEN)
    assert response.status_code == 400
    assert not Item.objects.exists()
    assert not Activity.objects.filter(kind="ingest").exists()


def test_encrypted_pdf_stores_blob_without_page_count(client, blob_dir):
    response = client.post(
        "/ingest/", {"file": pdf(ENCRYPTED_PDF, name="secret.pdf")}, **TOKEN
    )
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.document"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert "$blob" in values
    assert "pages" not in values
    assert (blob_dir / values["$blob"][:2]).exists()


def test_large_pdf_is_accepted(client, blob_dir):
    padded = TINY_PDF.read_bytes() + b"\n%" + (b"x" * 64_000)
    huge = SimpleUploadedFile("big.pdf", padded, "application/pdf")
    response = client.post("/ingest/", {"file": huge}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.document"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert values["pages"] == 1
    assert "$blob" in values


def test_mp4_becomes_video_with_duration_and_size(client, blob_dir):
    response = client.post("/ingest/", {"file": video(), "title": "lamp"}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.video"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert values["duration"] == 1
    assert (values["width"], values["height"]) == (2, 2)
    assert values["title"] == "lamp"
    assert (blob_dir / values["$blob"][:2]).exists()
    ranks = {
        claim.path: claim.rank
        for claim in Claim.objects.filter(
            item=item, path__in=("duration", "width", "height", "title")
        )
    }
    assert ranks == {
        "duration": Claim.Rank.NORMAL,
        "width": Claim.Rank.NORMAL,
        "height": Claim.Rank.NORMAL,
        "title": Claim.Rank.PREFERRED,
    }


@pytest.mark.parametrize(
    ("name", "content_type"),
    [
        ("clip.mov", "video/quicktime"),
        ("clip.mp4", "application/octet-stream"),
    ],
)
def test_quicktime_and_octet_stream_mp4_are_video(client, blob_dir, name, content_type):
    response = client.post(
        "/ingest/", {"file": video(name=name, content_type=content_type)}, **TOKEN
    )
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.video"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert values["duration"] == 1
    assert (values["width"], values["height"]) == (2, 2)
    assert "$blob" in values


def test_audio_only_video_keeps_blob_and_duration(client, blob_dir):
    upload = SimpleUploadedFile("hum.mp4", audio_only_mp4(), "video/mp4")
    response = client.post("/ingest/", {"file": upload}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.video"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert values["duration"] == 0.5
    assert "$blob" in values
    assert "width" not in values
    assert "height" not in values
    assert (blob_dir / values["$blob"][:2]).exists()


def test_unknown_video_bytes_still_store_a_blob(client, blob_dir):
    junk = SimpleUploadedFile("clip.webm", b"not a container", "video/webm")
    response = client.post("/ingest/", {"file": junk}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.video"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert "$blob" in values
    assert "duration" not in values
    assert (blob_dir / values["$blob"][:2]).exists()


def test_large_video_is_accepted(client, blob_dir):
    padded = TINY_MP4.read_bytes() + (b"\x00" * 64_000)
    huge = SimpleUploadedFile("big.mp4", padded, "video/mp4")
    response = client.post("/ingest/", {"file": huge}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.video"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert values["duration"] == 1
    assert "$blob" in values


def test_same_bytes_twice_are_two_items_one_blob(client, blob_dir):
    payload = b"same lamp twice"

    def upload():
        return SimpleUploadedFile("lamp.bin", payload, "application/octet-stream")

    first = client.post("/ingest/", {"file": upload()}, **TOKEN)
    second = client.post("/ingest/", {"file": upload()}, **TOKEN)
    assert first.status_code == 201, first.content
    assert second.status_code == 201, second.content
    assert first.json()["id"] != second.json()["id"]
    assert Item.objects.count() == 2
    assert Blob.objects.count() == 1
    sha = Item.objects.get(id=first.json()["id"]).current_values()["$blob"]
    assert Item.objects.get(id=second.json()["id"]).current_values()["$blob"] == sha
    assert (blob_dir / sha[:2] / sha).read_bytes() == payload


def test_over_limit_body_is_413(client, settings):
    settings.TROVE = {**settings.TROVE, "INGEST_MAX_BYTES": 64}
    huge = SimpleUploadedFile("big.bin", b"x" * 1000, "application/octet-stream")
    response = client.post("/ingest/", {"file": huge}, **TOKEN)
    assert response.status_code == 413
    assert not Item.objects.exists()
    assert not Blob.objects.exists()
    assert not Activity.objects.filter(kind="ingest").exists()


def test_live_photo_keeps_the_still_and_ignores_motion(client, blob_dir):
    still = SimpleUploadedFile("IMG_0001.HEIC", TINY_HEIC.read_bytes(), "image/heic")
    motion = SimpleUploadedFile(
        "IMG_0001.MOV", TINY_MP4.read_bytes(), "video/quicktime"
    )
    response = client.post(
        "/ingest/",
        {"file": still, "video": motion, "title": "hallway"},
        **TOKEN,
    )
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.image"
    assert Item.objects.count() == 1
    values = Item.objects.get(id=payload["id"]).current_values()
    assert values["title"] == "hallway"
    assert (blob_dir / values["$blob"][:2] / values["$blob"]).read_bytes() == (
        TINY_HEIC.read_bytes()
    )


def test_live_photo_motion_only_is_video(client, blob_dir):
    motion = SimpleUploadedFile(
        "IMG_0001.MOV", TINY_MP4.read_bytes(), "video/quicktime"
    )
    response = client.post("/ingest/", {"file": motion}, **TOKEN)
    assert response.status_code == 201, response.content
    assert response.json()["type"] == "capture.video"
    values = Item.objects.get(id=response.json()["id"]).current_values()
    assert "$blob" in values
    assert (blob_dir / values["$blob"][:2]).exists()


def test_ingest_page_requires_session(client):
    assert client.get("/ingest/").status_code == 401


def test_owner_sees_ingest_form(client):
    client.force_login(ensure_owner())
    response = client.get("/ingest/")
    assert response.status_code == 200
    assert response["Content-Type"].startswith("text/html")
    body = response.content.decode()
    assert 'name="file"' in body
    assert 'name="url"' in body
    assert 'name="title"' in body
    assert 'name="notes"' in body
    assert "csrfmiddlewaretoken" in body


def test_owner_session_posts_url_with_preferred_claims(client):
    client.force_login(ensure_owner())
    response = client.post(
        "/ingest/",
        {
            "url": "https://example.com/lamp",
            "title": "hallway",
            "notes": "warm",
        },
    )
    assert response.status_code == 302
    item_id = parse_qs(urlparse(response["Location"]).query)["id"][0]
    item = Item.objects.get(id=item_id)
    assert item.type == "capture.link"
    values = item.current_values()
    assert values["url"] == "https://example.com/lamp"
    assert values["title"] == "hallway"
    assert values["notes"] == "warm"
    ranks = {
        claim.path: claim.rank
        for claim in Claim.objects.filter(item=item, path__in=("url", "title", "notes"))
    }
    assert ranks == {
        "url": Claim.Rank.PREFERRED,
        "title": Claim.Rank.PREFERRED,
        "notes": Claim.Rank.PREFERRED,
    }
    act = item.activity
    assert (act.kind, act.agent, act.outcome) == ("ingest", "owner", "ok")
    shown = client.get(response["Location"])
    assert shown.status_code == 200
    assert item_id in shown.content.decode()


def test_owner_session_posts_file(client, blob_dir):
    client.force_login(ensure_owner())
    response = client.post(
        "/ingest/",
        {"file": pdf(), "title": "spec", "notes": "from the browser"},
    )
    assert response.status_code == 302
    item_id = parse_qs(urlparse(response["Location"]).query)["id"][0]
    item = Item.objects.get(id=item_id)
    assert item.type == "capture.document"
    values = item.current_values()
    assert values["title"] == "spec"
    assert values["notes"] == "from the browser"
    assert (blob_dir / values["$blob"][:2]).exists()
    assert item.activity.agent == "owner"


def test_owner_session_post_needs_csrf():
    client = Client(enforce_csrf_checks=True)
    client.force_login(ensure_owner())
    denied = client.post("/ingest/", {"url": "https://example.com/csrf"})
    assert denied.status_code == 403
    assert not Item.objects.exists()
    page = client.get("/ingest/")
    assert page.status_code == 200
    token = client.cookies["csrftoken"].value
    created = client.post(
        "/ingest/",
        {"url": "https://example.com/csrf", "csrfmiddlewaretoken": token},
    )
    assert created.status_code == 302
    item_id = parse_qs(urlparse(created["Location"]).query)["id"][0]
    assert Item.objects.get(id=item_id).type == "capture.link"


class Fetched:
    def __init__(self, body, content_type, url):
        self._body = body
        self.headers = EmailMessage()
        self.headers["Content-Type"] = content_type
        self._url = url

    def __enter__(self):
        return self

    def __exit__(self, *args):
        return False

    def read(self, n=-1):
        if n is None or n < 0:
            return self._body
        return self._body[:n]

    def geturl(self):
        return self._url


def png_bytes():
    img = Image.new("RGB", (2, 2), "#336699")
    buf = io.BytesIO()
    img.save(buf, "PNG")
    return buf.getvalue()


def fake_urlopen(body, content_type, url):
    def urlopen(request, timeout=None):
        return Fetched(body, content_type, url)

    return urlopen


def test_page_url_does_not_fetch(client, monkeypatch):
    seen = []

    def urlopen(request, timeout=None):
        seen.append(request.full_url)
        raise AssertionError("ingest should not fetch a page url")

    monkeypatch.setattr("urllib.request.urlopen", urlopen)
    response = client.post(
        "/ingest/", {"url": "https://example.com/blog/post"}, **TOKEN
    )
    assert response.status_code == 201
    assert response.json()["type"] == "capture.link"
    assert seen == []


def test_public_pdf_url_becomes_document_keeping_preferred_url(
    client, blob_dir, monkeypatch
):
    url = "https://example.com/spec.pdf"
    monkeypatch.setattr(
        "urllib.request.urlopen",
        fake_urlopen(TINY_PDF.read_bytes(), "application/pdf", url),
    )
    response = client.post(
        "/ingest/", {"url": url, "title": "spec", "notes": "from the cdn"}, **TOKEN
    )
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.document"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert values["url"] == url
    assert values["title"] == "spec"
    assert values["notes"] == "from the cdn"
    assert values["pages"] == 1
    assert (blob_dir / values["$blob"][:2]).exists()
    assert Claim.objects.get(item=item, path="url").rank == Claim.Rank.PREFERRED
    assert item.activity.outcome == Activity.Outcome.OK
    assert not Activity.objects.filter(subject=item, outcome="failed").exists()


def test_public_image_url_becomes_image_keeping_preferred_url(
    client, blob_dir, monkeypatch
):
    url = "https://example.com/lamp.jpg"
    body = png_bytes()
    monkeypatch.setattr(
        "urllib.request.urlopen",
        fake_urlopen(body, "image/png", url),
    )
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.image"
    item = Item.objects.get(id=payload["id"])
    values = item.current_values()
    assert values["url"] == url
    assert (values["width"], values["height"]) == (2, 2)
    assert (blob_dir / values["$blob"][:2] / values["$blob"]).read_bytes() == body
    assert Claim.objects.get(item=item, path="url").rank == Claim.Rank.PREFERRED


def test_pdf_url_that_returns_html_stays_a_link(client, blob_dir, monkeypatch):
    url = "https://example.com/spec.pdf"
    monkeypatch.setattr(
        "urllib.request.urlopen",
        fake_urlopen(b"<html>login</html>", "text/html", url),
    )
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.link"
    item = Item.objects.get(id=payload["id"])
    assert item.current_values() == {"url": url}
    assert not Blob.objects.exists()


def http_404(request, timeout=None):
    raise HTTPError(request.full_url, 404, "Not Found", {}, io.BytesIO(b""))


def timed_out(request, timeout=None):
    raise URLError(TimeoutError("timed out"))


@pytest.mark.parametrize("fake", [http_404, timed_out])
def test_pdf_url_fetch_failure_keeps_the_link(client, monkeypatch, fake):
    monkeypatch.setattr("urllib.request.urlopen", fake)
    url = "https://example.com/spec.pdf"
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.link"
    item = Item.objects.get(id=payload["id"])
    assert item.current_values()["url"] == url
    assert item.activity.outcome == Activity.Outcome.OK
    failed = Activity.objects.get(subject=item, outcome="failed")
    assert failed.error
    assert not Blob.objects.exists()


@pytest.mark.parametrize(
    "url",
    [
        "file:///tmp/spec.pdf",
        "http://127.0.0.1/spec.pdf",
        "http://10.0.0.8/spec.pdf",
        "http://192.168.1.9/lamp.jpg",
        "http://100.64.1.2/lamp.jpg",
        "http://[::ffff:100.64.1.2]/lamp.jpg",
        "http://localhost/spec.pdf",
    ],
)
def test_file_or_private_url_is_not_fetched(client, monkeypatch, url):
    seen = []

    def urlopen(request, timeout=None):
        seen.append(request.full_url)
        raise AssertionError("must not fetch file or private urls")

    monkeypatch.setattr("urllib.request.urlopen", urlopen)
    response = client.post("/ingest/", {"url": url}, **TOKEN)
    assert response.status_code == 201, response.content
    payload = response.json()
    assert payload["type"] == "capture.link"
    item = Item.objects.get(id=payload["id"])
    assert item.current_values()["url"] == url
    assert item.activity.outcome == Activity.Outcome.OK
    assert Activity.objects.filter(subject=item, outcome="failed").exists()
    assert seen == []
