from pathlib import Path

import pytest
from django.core.files.uploadedfile import SimpleUploadedFile
from PIL import Image

from tests.factories import seed_dev_ingest_token
from trove.core import get_core, reset_core
from trove.models import Activity, Claim, Item, Link
from trove.tasks.execute import execute_derive

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


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


def pdf_file(path=TINY_PDF, name="spec.pdf", content_type="application/pdf"):
    return SimpleUploadedFile(name, path.read_bytes(), content_type)


def pdf_with_text(text: str) -> bytes:
    stream = f"BT /F1 24 Tf 72 720 Td ({text}) Tj ET\n".encode()
    objects = [
        b"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n",
        b"2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n",
        (
            b"3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R"
            b"/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>endobj\n"
        ),
        b"4 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n",
        f"5 0 obj<</Length {len(stream)}>>stream\n".encode()
        + stream
        + b"endstream\nendobj\n",
    ]
    out = b"%PDF-1.1\n"
    offsets = []
    for obj in objects:
        offsets.append(len(out))
        out += obj
    xref_pos = len(out)
    xref = ["xref\n0 6\n0000000000 65535 f \n"]
    for offset in offsets:
        xref.append(f"{offset:010d} 00000 n \n")
    out += "".join(xref).encode()
    out += (
        f"trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n{xref_pos}\n%%EOF\n".encode()
    )
    return out


def ingest_pdf(client, upload):
    response = client.post("/ingest/", {"file": upload, "title": "spec"}, **TOKEN)
    assert response.status_code == 201, response.content
    return Item.objects.get(id=response.json()["id"])


def derive_pdf(core, item):
    execute_derive(core.processors["pdf"], item.id)


def test_pdf_processor_consumes_documents(core):
    processor = core.processors["pdf"]
    assert processor.consumes == "capture.document"
    assert processor.provides == ["capture.image"]


def test_tiny_pdf_emits_a_first_page_image(client, blob_dir, core):
    item = ingest_pdf(client, pdf_file())
    derive_pdf(core, item)
    still = Item.objects.get(type="capture.image")
    values = still.current_values()
    sha = values["$blob"]
    with Image.open(blob_dir / sha[:2] / sha) as img:
        assert img.size[0] > 0
        assert img.size[1] > 0
    link = Link.objects.live().get(rel="derived_from")
    assert link.source_id == still.id
    assert link.target_id == item.id
    act = Activity.objects.get(agent="pdf@1", subject=item, outcome="ok")
    assert act.kind == "derive"
    assert "text" not in item.current_values()


def test_page_count_stays_the_ingest_claim(client, blob_dir, core):
    item = ingest_pdf(client, pdf_file())
    derive_pdf(core, item)
    pages = Claim.objects.filter(item=item, path="pages")
    assert pages.count() == 1
    claim = pages.get()
    assert claim.value == 1
    assert claim.activity.kind == "ingest"
    assert claim.activity.agent != "pdf@1"


def test_embedded_text_is_a_claim_on_the_document(client, blob_dir, core):
    upload = SimpleUploadedFile(
        "notes.pdf", pdf_with_text("hallway lamp"), "application/pdf"
    )
    item = ingest_pdf(client, upload)
    derive_pdf(core, item)
    assert item.current_values()["text"] == "hallway lamp"
    assert Item.objects.filter(type="capture.image").count() == 1


def test_encrypted_pdf_fails_derive_and_keeps_the_blob(client, blob_dir, core):
    item = ingest_pdf(client, pdf_file(ENCRYPTED_PDF, name="secret.pdf"))
    sha = item.current_values()["$blob"]
    original = (blob_dir / sha[:2] / sha).read_bytes()
    derive_pdf(core, item)
    act = Activity.objects.get(agent="pdf@1", subject=item)
    assert act.outcome == "failed"
    assert (blob_dir / sha[:2] / sha).read_bytes() == original
    assert not Item.objects.filter(type="capture.image").exists()
    assert "pages" not in item.current_values()
    assert "text" not in item.current_values()


def test_corrupt_pdf_fails_derive_and_keeps_the_blob(client, blob_dir, core):
    upload = SimpleUploadedFile("bad.pdf", b"%PDF-1.1\nnot a pdf", "application/pdf")
    item = ingest_pdf(client, upload)
    sha = item.current_values()["$blob"]
    original = (blob_dir / sha[:2] / sha).read_bytes()
    derive_pdf(core, item)
    act = Activity.objects.get(agent="pdf@1", subject=item)
    assert act.outcome == "failed"
    assert (blob_dir / sha[:2] / sha).read_bytes() == original
    assert not Item.objects.filter(type="capture.image").exists()


def test_palette_can_consume_the_still(client, blob_dir, core):
    item = ingest_pdf(client, pdf_file())
    derive_pdf(core, item)
    still = Item.objects.get(type="capture.image")
    execute_derive(core.processors["palette"], still.id)
    assert Item.objects.filter(type="colour").exists()
