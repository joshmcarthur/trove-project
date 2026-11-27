import json
from io import BytesIO
from urllib.error import HTTPError

import pytest


def completion_body(content="{}", cost=None, model="gpt-4o-mini"):
    usage = {}
    if cost is not None:
        usage["cost"] = cost
    return {
        "choices": [{"message": {"content": content}}],
        "model": model,
        "usage": usage,
    }


class FakeResponse:
    def __init__(self, payload=None):
        self._raw = json.dumps(payload or completion_body()).encode()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        return False

    def read(self):
        return self._raw


def http_error(code, headers=None):
    return HTTPError(
        "https://api.openai.com/v1/chat/completions",
        code,
        "error",
        headers or {},
        BytesIO(b""),
    )


@pytest.fixture(autouse=True)
def stub_model_http(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "test-key")
    monkeypatch.setattr(
        "urllib.request.urlopen", lambda *args, **kwargs: FakeResponse()
    )
