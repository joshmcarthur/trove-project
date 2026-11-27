from pathlib import Path

import pytest
from django.core.files.uploadedfile import SimpleUploadedFile

from tests.factories import seed_dev_ingest_token
from trove.models import Claim, Item

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}
FIXTURES = Path(__file__).parent / "fixtures"
TINY_PDF = FIXTURES / "tiny.pdf"
ENCRYPTED_PDF = FIXTURES / "encrypted.pdf"

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
