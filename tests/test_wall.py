import io

import pytest
from django.urls import reverse
from PIL import Image

from tests.factories import ItemFactory, seed_dev_ingest_token, seed_dev_wall_token
from trove.models import Claim
from trove.services.clients import ensure_wall_client
from trove.services.recorder import record

pytestmark = pytest.mark.django_db(databases=["default", "data"])
AUTH = {"HTTP_AUTHORIZATION": "Bearer dev-wall-token"}


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def image_bytes(fmt="PNG"):
    buf = io.BytesIO()
    Image.new("RGB", (8, 8), "#336699").save(buf, fmt)
    return buf.getvalue()


def store_blob(type_name, data, content_type, kind="ingest"):
    with record(kind, "test") as rec:
        item = rec.item(type_name)
        sha = rec.blob(data, content_type)
        rec.claim(item, "$blob", sha)
        rec.claim(item, "title", "hallway lamp")
    return item, sha


def test_wall_page_serves_spa_without_a_token(client):
    app = ensure_wall_client()
    response = client.get("/wall/")
    assert response.status_code == 200
    assert response["Content-Type"].startswith("text/html")
    body = response.content.decode()
    assert app.client_id in body
    assert 'src="vendor/wall.js"' in body
    assert 'href="vendor/wall.css"' in body
    assert reverse("oauth2_provider:authorize") in body
    assert reverse("oauth2_provider:token") in body
    assert "plugin enablement" not in body.lower()
    assert "budget" not in body.lower()


def test_wall_vendor_serves_preact_htm_and_app(client):
    js = client.get("/wall/vendor/preact.module.js")
    assert js.status_code == 200
    assert "javascript" in js["Content-Type"]
    assert b"export{" in js.content

    htm = client.get("/wall/vendor/htm.module.js")
    assert htm.status_code == 200
    assert b"export default" in htm.content

    app = client.get("/wall/vendor/wall.js")
    assert app.status_code == 200
    text = app.content.decode()
    assert "sessionStorage" in text
    assert "localStorage" not in text
    assert "document.cookie" not in text
    assert "/wall/api/items" in text
    assert "/wall/api/blobs/" in text

    css = client.get("/wall/vendor/wall.css")
    assert css.status_code == 200
    assert "css" in css["Content-Type"]


def test_wall_vendor_unknown_file_is_404(client):
    assert client.get("/wall/vendor/secret.py").status_code == 404
    assert client.get("/wall/vendor/../queries.py").status_code == 404


def test_wall_items_include_current_values_and_provenance(client):
    item = ItemFactory()
    with record("edit", "user") as rec:
        rec.claim(item, "title", "hallway lamp", rank=Claim.Rank.PREFERRED)
    seed_dev_wall_token()
    payload = client.get(
        "/wall/api/items", HTTP_AUTHORIZATION="Bearer dev-wall-token"
    ).json()["items"]
    row = next(entry for entry in payload if entry["id"] == item.id)
    assert row["values"]["title"] == "hallway lamp"
    assert row["colours"] == []
    agents = {entry["agent"] for entry in row["provenance"]}
    kinds = {entry["kind"] for entry in row["provenance"]}
    assert "user" in agents
    assert "edit" in kinds
    assert all("when" in entry for entry in row["provenance"])


def test_wall_blob_returns_png_bytes_and_content_type(client, blob_dir):
    data = image_bytes("PNG")
    item, sha = store_blob("capture.image", data, "image/png")
    seed_dev_wall_token()
    response = client.get(f"/wall/api/blobs/{sha}", **AUTH)
    assert response.status_code == 200
    assert response["Content-Type"].startswith("image/png")
    assert b"".join(response.streaming_content) == data
    payload = client.get("/wall/api/items", **AUTH).json()["items"]
    row = next(entry for entry in payload if entry["id"] == item.id)
    assert row["values"]["$blob"] == sha


def test_wall_blob_returns_jpeg_derived_still(client, blob_dir):
    data = image_bytes("JPEG")
    item, sha = store_blob("capture.image", data, "image/jpeg", kind="derive")
    seed_dev_wall_token()
    response = client.get(f"/wall/api/blobs/{sha}", **AUTH)
    assert response.status_code == 200
    assert response["Content-Type"].startswith("image/jpeg")
    assert b"".join(response.streaming_content) == data
    assert item.type == "capture.image"


def test_wall_blob_rejects_missing_auth(client, blob_dir):
    data = image_bytes("PNG")
    _item, sha = store_blob("capture.image", data, "image/png")
    response = client.get(f"/wall/api/blobs/{sha}")
    assert response.status_code == 401
    assert response.json() == {"error": "unauthorised"}


def test_wall_blob_rejects_write_capture_token(client, blob_dir):
    data = image_bytes("PNG")
    _item, sha = store_blob("capture.image", data, "image/png")
    seed_dev_ingest_token()
    response = client.get(
        f"/wall/api/blobs/{sha}", HTTP_AUTHORIZATION="Bearer dev-shortcut-token"
    )
    assert response.status_code == 401


def test_wall_blob_unknown_sha_is_404_when_signed_in(client):
    seed_dev_wall_token()
    sha = "a" * 64
    response = client.get(f"/wall/api/blobs/{sha}", **AUTH)
    assert response.status_code == 404


def test_wall_spa_shows_photo_for_image_and_grey_tile_otherwise(client):
    text = client.get("/wall/vendor/wall.js").content.decode()
    assert "function photoSha" in text
    assert 'item.type !== "capture.image"' in text
    assert "tile-empty" in text
    assert 'img class="tile"' in text
    assert "createObjectURL" in text
    assert "capture.video" not in text
    assert "localStorage" not in text
    assert "document.cookie" not in text
