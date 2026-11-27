import pytest

from tests.factories import seed_dev_ingest_token
from trove.models import Claim, Item

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture(autouse=True)
def ingest_token():
    seed_dev_ingest_token()


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
