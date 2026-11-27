import urllib.request
from email.message import EmailMessage
from io import BytesIO
from urllib.parse import urlparse
from urllib.response import addinfourl

import pytest

from trove.services.fetch_error import FetchError
from trove.services.public_http import fetch_public_http, make_opener


class RedirectToPrivate(urllib.request.BaseHandler):
    handler_order = 1

    def __init__(self):
        self.opened = []

    def http_open(self, req):
        return self._open(req)

    def https_open(self, req):
        return self._open(req)

    def _open(self, req):
        url = req.full_url
        self.opened.append(url)
        host = urlparse(url).hostname
        if host in {"127.0.0.1", "10.0.0.1", "::1"}:
            raise AssertionError(f"second hop opened: {url}")
        headers = EmailMessage()
        headers["Location"] = "http://127.0.0.1/secret.pdf"
        resp = addinfourl(BytesIO(b""), headers, url, code=302)
        resp.msg = "Found"
        return resp


def test_redirect_to_private_ip_is_not_opened(monkeypatch):
    handler = RedirectToPrivate()
    opener = make_opener(handler)

    def open_http(request, timeout=None):
        return opener.open(request, timeout=timeout)

    monkeypatch.setattr("trove.services.public_http.urlopen", open_http)
    with pytest.raises(FetchError, match="private"):
        fetch_public_http(
            "https://example.com/spec.pdf",
            max_bytes=1024,
            user_agent="Trove/0.1 (test)",
        )
    assert handler.opened == ["https://example.com/spec.pdf"]
    assert all("127.0.0.1" not in url for url in handler.opened)


def test_opener_omits_file_and_ftp_handlers():
    kinds = {type(h) for h in make_opener().handlers}
    assert urllib.request.FileHandler not in kinds
    assert urllib.request.FTPHandler not in kinds
