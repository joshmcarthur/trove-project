import pytest
from django.urls import reverse

from tests.factories import ItemFactory, seed_dev_wall_token
from trove.models import Claim
from trove.services.clients import ensure_wall_client
from trove.services.recorder import record

pytestmark = pytest.mark.django_db(databases=["default", "data"])


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
