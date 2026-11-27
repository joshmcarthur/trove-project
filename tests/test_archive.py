import io
from email.message import EmailMessage
from pathlib import Path
from urllib.error import HTTPError, URLError

import pytest

from tests.factories import ItemFactory
from trove.core import get_core, reset_core
from trove.models import Activity, Blob, Claim, Item, Link
from trove.services.recorder import record
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])

PAGE = "https://example.com/lamp"
FIXTURE = Path(__file__).parent / "fixtures" / "lamp.mhtml"
MHTML = FIXTURE.read_bytes()
PUBLIC_ADDR = [
    (0, 0, 0, "", ("93.184.216.34", 0)),
]


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


@pytest.fixture
def public_dns(monkeypatch):
    monkeypatch.setattr("socket.getaddrinfo", lambda *a, **k: PUBLIC_ADDR)


class Fetched:
    def __init__(self, body, content_type="multipart/related", url=PAGE):
        self._body = body if isinstance(body, bytes) else body.encode()
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


def make_link(url):
    with record("ingest", "test") as rec:
        item = rec.item("capture.link")
        rec.claim(item, "url", url, rank=Claim.Rank.PREFERRED)
    return item


def returns(body, url=PAGE, timeout_seen=None):
    def urlopen(request, timeout=None):
        if timeout_seen is not None:
            timeout_seen.append(timeout)
        assert not request.has_header("Authorization")
        return Fetched(body, url=url)

    return urlopen


def http_404(request, timeout=None):
    raise HTTPError(request.full_url, 404, "Not Found", {}, io.BytesIO(b""))


def timed_out(request, timeout=None):
    raise URLError(TimeoutError("timed out"))


def archive(core, item):
    execute_derive(core.processors["archive"], item.id)
    item.refresh_from_db()
    return item


def derived_files(link):
    ids = (
        Link.objects.live().filter(target=link, rel="derived_from").values("source_id")
    )
    return list(Item.objects.filter(id__in=ids, type="capture.file"))


def test_missing_url_fails_and_keeps_the_item(core):
    item = ItemFactory(type="capture.link")
    archive(core, item)
    act = Activity.objects.get(agent="archive@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id).exists()
    assert not Item.objects.filter(type="capture.file").exists()


def test_public_https_url_stores_mhtml_blob(core, blob_dir, public_dns, monkeypatch):
    timeouts = []
    monkeypatch.setattr("urllib.request.urlopen", returns(MHTML, timeout_seen=timeouts))
    item = archive(core, make_link(PAGE))
    files = derived_files(item)
    assert len(files) == 1
    derived = files[0]
    values = derived.current_values()
    sha = values["$blob"]
    assert values["$key"] == "mhtml"
    assert Blob.objects.get(pk=sha).content_type == "multipart/related"
    assert (blob_dir / sha[:2] / sha).read_bytes() == MHTML
    assert (
        Link.objects.live()
        .filter(source=derived, target=item, rel="derived_from")
        .exists()
    )
    url_claim = Claim.objects.get(item=item, path="url")
    assert url_claim.value == PAGE
    assert url_claim.rank == Claim.Rank.PREFERRED
    act = Activity.objects.get(agent="archive@1", subject=item)
    assert act.outcome == "ok"
    assert timeouts == [30]


def test_same_url_twice_gets_two_archive_items(core, blob_dir, public_dns, monkeypatch):
    monkeypatch.setattr("urllib.request.urlopen", returns(MHTML))
    first = archive(core, make_link(PAGE))
    second = archive(core, make_link(PAGE))
    assert first.id != second.id
    first_files = derived_files(first)
    second_files = derived_files(second)
    assert len(first_files) == 1
    assert len(second_files) == 1
    assert first_files[0].id != second_files[0].id


def test_rederive_reuses_the_same_archive_item(core, blob_dir, public_dns, monkeypatch):
    monkeypatch.setattr("urllib.request.urlopen", returns(MHTML))
    item = archive(core, make_link(PAGE))
    first = derived_files(item)[0]
    archive(core, item)
    files = derived_files(item)
    assert [row.id for row in files] == [first.id]


@pytest.mark.parametrize(
    "url",
    [
        "file:///etc/passwd",
        "data:text/html,hi",
        "http://127.0.0.1/",
        "http://10.0.0.1/",
        "http://100.64.0.1/",
        "http://169.254.1.1/",
    ],
)
def test_private_urls_fail_without_fetching(core, monkeypatch, url):
    seen = []

    def urlopen(request, timeout=None):
        seen.append(request.full_url)
        return Fetched(MHTML)

    monkeypatch.setattr("urllib.request.urlopen", urlopen)
    item = archive(core, make_link(url))
    assert seen == []
    act = Activity.objects.get(agent="archive@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


def test_oversize_archive_fails_without_storing(
    core, blob_dir, public_dns, monkeypatch
):
    monkeypatch.setattr(
        "urllib.request.urlopen", returns(b"x" * (20 * 1024 * 1024 + 1))
    )
    item = archive(core, make_link(PAGE))
    act = Activity.objects.get(agent="archive@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()


@pytest.mark.parametrize("fake", [http_404, timed_out])
def test_fetch_failures_fail_derive_and_keep_the_link(
    core, public_dns, monkeypatch, fake
):
    monkeypatch.setattr("urllib.request.urlopen", fake)
    item = archive(core, make_link(PAGE))
    act = Activity.objects.get(agent="archive@1", subject=item)
    assert act.outcome == "failed"
    assert Item.objects.filter(id=item.id, type="capture.link").exists()
    assert not Item.objects.filter(type="capture.file").exists()
    assert not Blob.objects.exists()
