"""Redirect handler that checks each hop before following it."""

from urllib.request import HTTPRedirectHandler

from trove.services.public_url import require_public_http


class PublicRedirectHandler(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        require_public_http(newurl)
        return super().redirect_request(req, fp, code, msg, headers, newurl)
