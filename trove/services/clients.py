import os

from oauth2_provider.models import AbstractApplication, get_application_model

WALL_CLIENT_NAME = "wall"
_DEFAULT_WALL_REDIRECT_URIS = (
    "http://localhost:8123/wall/",
    "http://127.0.0.1:8123/wall/",
    "http://testserver/wall/",
)


def wall_redirect_uris() -> str:
    extra = os.environ.get("TROVE_WALL_REDIRECT_URI", "").strip()
    uris = list(_DEFAULT_WALL_REDIRECT_URIS)
    if extra and extra not in uris:
        uris.append(extra)
    return " ".join(uris)


def ensure_wall_client():
    Application = get_application_model()
    uris = wall_redirect_uris()
    app = Application.objects.filter(name=WALL_CLIENT_NAME).first()
    if app is None:
        return Application.objects.create(
            name=WALL_CLIENT_NAME,
            client_type=Application.CLIENT_PUBLIC,
            authorization_grant_type=Application.GRANT_AUTHORIZATION_CODE,
            redirect_uris=uris,
            skip_authorization=False,
            hash_client_secret=False,
            client_secret="",
        )
    if app.redirect_uris != uris:
        app.redirect_uris = uris
        app.save(update_fields=["redirect_uris"])
    return app


def application_for_name(name: str, owner):
    Application = get_application_model()
    app = Application.objects.filter(name=name).first()
    if app is not None:
        return app
    return Application.objects.create(
        name=name,
        user=owner,
        client_type=Application.CLIENT_CONFIDENTIAL,
        authorization_grant_type=AbstractApplication.GRANT_CLIENT_CREDENTIALS,
        redirect_uris="",
        skip_authorization=True,
    )
