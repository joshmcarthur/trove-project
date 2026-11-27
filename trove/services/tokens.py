import hashlib
from datetime import timedelta

from django.utils import timezone
from oauth2_provider.models import get_access_token_model, set_token_value
from oauth2_provider.utils import parse_bearer_token
from oauthlib.common import generate_token

from trove.services.clients import application_for_name
from trove.services.owner import get_owner

INGEST_SCOPE = "write:capture"
LONG_LIVED = timedelta(days=3650)


def mint_ingest_token(client_name: str, raw_token: str | None = None) -> str:
    owner = get_owner()
    if owner is None:
        raise ValueError("Register the owner passkey at /setup first.")
    application = application_for_name(client_name, owner=owner)
    raw = raw_token or generate_token()
    token = get_access_token_model()(
        user=owner,
        application=application,
        expires=timezone.now() + LONG_LIVED,
        scope=INGEST_SCOPE,
    )
    set_token_value(token, raw)
    token.save()
    return raw


def agent_for_request(request, required_scope: str) -> str | None:
    raw = parse_bearer_token(request.headers.get("Authorization", ""))
    if not raw:
        return None
    checksum = hashlib.sha256(raw.encode("utf-8")).hexdigest()
    token = (
        get_access_token_model()
        .objects.select_related("application")
        .filter(token_checksum=checksum)
        .first()
    )
    if token is None or token.application is None:
        return None
    if not token.is_valid([required_scope]):
        return None
    return token.application.name
