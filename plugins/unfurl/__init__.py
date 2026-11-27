"""Builtin plugin: fetch a link's title and preview image."""

from html.parser import HTMLParser
from urllib.parse import urljoin

from trove.core import Processor
from trove.services.fetch_error import FetchError
from trove.services.public_http import fetch_public_http
from trove.tasks import Emit, Result

MAX_BYTES = 1_048_576
USER_AGENT = "Trove/0.1 (unfurl)"
HTML_TYPES = {"text/html", "application/xhtml+xml"}


class UnfurlError(Exception):
    pass


class _OgParser(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.og_title = None
        self.twitter_title = None
        self.title = None
        self.og_image = None
        self.twitter_image = None
        self._in_title = False
        self._title_chunks: list[str] = []

    def handle_starttag(self, tag, attrs):
        attrs = {key.lower(): value for key, value in attrs}
        if tag == "title":
            self._in_title = True
            return
        if tag != "meta":
            return
        prop = (attrs.get("property") or attrs.get("name") or "").lower()
        content = (attrs.get("content") or "").strip()
        if not content:
            return
        if prop == "og:title":
            self.og_title = content
        elif prop == "twitter:title":
            self.twitter_title = content
        elif prop in {"og:image", "og:image:url"} and self.og_image is None:
            self.og_image = content
        elif prop == "twitter:image" and self.twitter_image is None:
            self.twitter_image = content

    def handle_endtag(self, tag):
        if tag != "title" or not self._in_title:
            return
        self._in_title = False
        text = "".join(self._title_chunks).strip()
        if text and self.title is None:
            self.title = text

    def handle_data(self, data):
        if self._in_title:
            self._title_chunks.append(data)


def run(item, ctx):
    url = ctx.values(item).get("url")
    if not isinstance(url, str) or not url.strip():
        raise UnfurlError("link has no url")
    url = url.strip()
    body, content_type, final_url, charset = _get(url)
    if content_type not in HTML_TYPES:
        raise UnfurlError("non-HTML")
    title, image_url = _parse(body, charset)
    claims = {}
    if title:
        claims["title"] = title
    emits = []
    preview = _preview(image_url, final_url)
    if preview is not None:
        data, image_type = preview
        extra = {"title": title} if title else {}
        emits.append(
            Emit(
                type="capture.image",
                key="og:image",
                claims=extra,
                blob=data,
                content_type=image_type,
            )
        )
    return Result(claims=claims, emits=emits)


def setup(core, entry):
    core.add_processor(Processor("unfurl", 1, "capture.link", ["capture.image"], run))


def _get(url: str) -> tuple[bytes, str, str, str]:
    try:
        data, content_type, final_url, charset = fetch_public_http(
            url, max_bytes=MAX_BYTES, user_agent=USER_AGENT
        )
    except FetchError as exc:
        raise UnfurlError(str(exc)) from exc
    if len(data) > MAX_BYTES:
        data = data[:MAX_BYTES]
    return data, content_type, final_url, charset


def _parse(body: bytes, charset: str) -> tuple[str | None, str | None]:
    parser = _OgParser()
    parser.feed(body.decode(charset, errors="replace"))
    parser.close()
    title = parser.og_title or parser.twitter_title or parser.title
    if title:
        title = " ".join(title.split())
    image = parser.og_image or parser.twitter_image
    return title, image


def _preview(image_url: str | None, final_url: str):
    if not image_url:
        return None
    url = urljoin(final_url, image_url)
    try:
        body, content_type, _final, _charset = _get(url)
    except UnfurlError:
        return None
    if not content_type.startswith("image/") or not body:
        return None
    return body, content_type
