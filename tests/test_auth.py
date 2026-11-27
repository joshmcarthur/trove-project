import base64
import hashlib
import io
import json
import secrets
from io import StringIO
from unittest.mock import MagicMock, patch
from urllib.parse import parse_qs, urlparse

import pytest
from allauth.mfa.models import Authenticator
from asgiref.sync import async_to_sync
from asgiref.testing import ApplicationCommunicator
from django.contrib.auth import get_user_model
from django.core.management import call_command
from django.core.management.base import CommandError
from django.urls import reverse
from django.utils.functional import SimpleLazyObject
from oauth2_provider.models import get_access_token_model, get_application_model
from PIL import Image

from config.asgi import application
from tests.factories import seed_dev_ingest_token, seed_dev_wall_token
from trove.core import get_core, reset_core
from trove.models import Item
from trove.routers import TroveRouter
from trove.services.clients import ensure_wall_client
from trove.services.owner import ensure_owner, get_owner
from trove.services.setup_code import issue_setup_code

pytestmark = pytest.mark.django_db(databases=["default", "data"])


def _pkce_pair():
    verifier = base64.urlsafe_b64encode(secrets.token_bytes(32)).rstrip(b"=").decode()
    digest = hashlib.sha256(verifier.encode()).digest()
    challenge = base64.urlsafe_b64encode(digest).rstrip(b"=").decode()
    return verifier, challenge


def test_setup_code_printed_and_shown_again():
    first = StringIO()
    call_command("setup_code", stdout=first)
    assert "Setup code:" in first.getvalue()
    again = StringIO()
    call_command("setup_code", stdout=again)
    assert first.getvalue() == again.getvalue()


def test_setup_rejects_wrong_code(client):
    issue_setup_code()
    response = client.post(reverse("setup"), {"code": "nope-nope"})
    assert response.status_code == 400


def test_setup_registers_passkey_and_kills_the_code(client):
    code = issue_setup_code()
    assert client.get(reverse("setup")).status_code == 200
    shown = client.post(reverse("setup"), {"code": code})
    assert shown.status_code == 200
    assert b"Register the owner passkey" in shown.content

    fake_credential = {
        "id": "cred",
        "rawId": "cred",
        "type": "public-key",
        "response": {"clientDataJSON": "", "attestationObject": ""},
        "clientExtensionResults": {"credProps": {"rk": True}},
    }
    binding = MagicMock()
    binding.is_user_verified.return_value = True
    with (
        patch("allauth.mfa.webauthn.internal.auth.parse_registration_response"),
        patch(
            "allauth.mfa.webauthn.internal.auth.complete_registration",
            return_value=binding,
        ),
    ):
        done = client.post(
            reverse("setup"),
            {
                "name": "owner key",
                "credential": json.dumps(fake_credential),
            },
        )
    assert done.status_code == 302
    owner = get_owner()
    assert Authenticator.objects.filter(
        user=owner, type=Authenticator.Type.WEBAUTHN
    ).exists()
    reused = client.post(reverse("setup"), {"code": code})
    assert reused.status_code == 409


def test_token_create_mints_write_capture_and_names_the_agent(client):
    ensure_owner()
    out = StringIO()
    call_command("token", "create", "shortcut:iphone", stdout=out)
    raw = out.getvalue().strip()
    assert raw
    png = _tiny_png()
    created = client.post(
        "/ingest/",
        {"file": png, "title": "from shortcut"},
        HTTP_AUTHORIZATION=f"Bearer {raw}",
    )
    assert created.status_code == 201
    from trove.models import Item

    item = Item.objects.get(id=created.json()["id"])
    assert item.activity.agent == "shortcut:iphone"


def test_token_create_before_setup_fails():
    with pytest.raises(CommandError, match="/setup"):
        call_command("token", "create", "shortcut:iphone")


def test_ingest_rejects_read_scope_token(client):
    ensure_owner()
    seed_dev_ingest_token()
    token = get_access_token_model().objects.get()
    token.scope = "read:all"
    token.save(update_fields=["scope"])
    response = client.post(
        "/ingest/",
        {"url": "https://example.com"},
        HTTP_AUTHORIZATION="Bearer dev-shortcut-token",
    )
    assert response.status_code == 401


def test_tokens_listed_and_revoked_in_admin(client):
    ensure_owner()
    seed_dev_ingest_token()
    owner = get_owner()
    client.force_login(owner)
    token = get_access_token_model().objects.get()
    listing = client.get(reverse("admin:oauth2_provider_accesstoken_changelist"))
    assert listing.status_code == 200
    assert b"shortcut:iphone" in listing.content
    revoked = client.post(
        reverse("admin:oauth2_provider_accesstoken_changelist"),
        {"action": "revoke_tokens", "_selected_action": [str(token.pk)]},
    )
    assert revoked.status_code == 302
    assert not get_access_token_model().objects.filter(pk=token.pk).exists()


def test_wall_authorization_code_pkce(client):
    owner = ensure_owner()
    client.force_login(owner)
    app = ensure_wall_client()
    verifier, challenge = _pkce_pair()
    authorize = client.get(
        reverse("oauth2_provider:authorize"),
        {
            "response_type": "code",
            "client_id": app.client_id,
            "redirect_uri": "http://testserver/wall/",
            "scope": "read:all",
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "state": "xyz",
        },
    )
    assert authorize.status_code == 200
    form = authorize.context["form"]
    payload = {name: form[name].value() or "" for name in form.fields}
    payload["allow"] = True
    allowed = client.post(reverse("oauth2_provider:authorize"), payload)
    assert allowed.status_code == 302
    code = parse_qs(urlparse(allowed["Location"]).query)["code"][0]
    token = client.post(
        reverse("oauth2_provider:token"),
        {
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": "http://testserver/wall/",
            "client_id": app.client_id,
            "code_verifier": verifier,
        },
    )
    assert token.status_code == 200
    body = token.json()
    assert "access_token" in body
    saved = get_access_token_model().objects.get(token=body["access_token"])
    assert saved.application.name == "wall"
    assert saved.allow_scopes(["read:all"])


def test_wall_authorize_without_pkce_is_rejected(client):
    owner = ensure_owner()
    client.force_login(owner)
    app = ensure_wall_client()
    response = client.get(
        reverse("oauth2_provider:authorize"),
        {
            "response_type": "code",
            "client_id": app.client_id,
            "redirect_uri": "http://testserver/wall/",
            "scope": "read:all",
        },
    )
    assert response.status_code in {302, 400}
    if response.status_code == 302:
        assert "error=" in response["Location"]


def test_wall_items_rejects_missing_auth(client):
    response = client.get("/wall/api/items")
    assert response.status_code == 401


def test_wall_items_rejects_write_capture_token(client):
    seed_dev_ingest_token()
    response = client.get(
        "/wall/api/items", HTTP_AUTHORIZATION="Bearer dev-shortcut-token"
    )
    assert response.status_code == 401


def test_wall_items_accepts_read_all_token(client):
    seed_dev_wall_token()
    response = client.get("/wall/api/items", HTTP_AUTHORIZATION="Bearer dev-wall-token")
    assert response.status_code == 200
    assert "items" in response.json()


def test_wall_redirect_uris_point_at_wall_and_accept_env_host(monkeypatch):
    monkeypatch.setenv("TROVE_WALL_REDIRECT_URI", "http://100.64.0.1:8123/wall/")
    app = ensure_wall_client()
    uris = app.redirect_uris.split()
    assert "http://localhost:8123/wall/" in uris
    assert "http://127.0.0.1:8123/wall/" in uris
    assert "http://testserver/wall/" in uris
    assert "http://100.64.0.1:8123/wall/" in uris
    assert "http://localhost:8123/" not in uris
    assert "http://testserver/" not in uris


def test_loading_wall_plugin_ensures_client():
    get_application_model().objects.filter(name="wall").delete()
    reset_core()
    get_core()
    assert get_application_model().objects.filter(name="wall").exists()


def test_asgi_serves_webauthn_js():
    async def fetch():
        communicator = ApplicationCommunicator(
            application,
            {
                "type": "http",
                "asgi": {"version": "3.0"},
                "http_version": "1.1",
                "method": "GET",
                "scheme": "http",
                "path": "/static/mfa/js/webauthn.js",
                "raw_path": b"/static/mfa/js/webauthn.js",
                "query_string": b"",
                "headers": [(b"host", b"testserver")],
                "client": ("127.0.0.1", 123),
                "server": ("testserver", 80),
            },
        )
        await communicator.send_input(
            {"type": "http.request", "body": b"", "more_body": False}
        )
        start = await communicator.receive_output()
        await communicator.wait()
        return start["status"]

    assert async_to_sync(fetch)() == 200


def test_admin_login_redirects_to_allauth(client):
    response = client.get("/admin/login/")
    assert response.status_code == 302
    assert response["Location"].startswith("/accounts/login/")


def test_allow_relation_unwraps_lazy_objects_via_meta():
    item = Item(type="capture.image")
    lazy = SimpleLazyObject(lambda: item)
    router = TroveRouter()
    assert router.allow_relation(lazy, item) is True
    user = get_user_model()(username="owner")
    assert router.allow_relation(lazy, user) is False


def _tiny_png():
    buf = io.BytesIO()
    Image.new("RGB", (2, 2), "#336699").save(buf, "PNG")
    buf.name = "x.png"
    buf.seek(0)
    return buf
