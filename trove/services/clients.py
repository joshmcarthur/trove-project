from oauth2_provider.models import AbstractApplication, get_application_model

WALL_CLIENT_NAME = "wall"
WALL_REDIRECT_URIS = " ".join(
    [
        "http://localhost:8123/",
        "http://127.0.0.1:8123/",
        "http://testserver/",
    ]
)


def ensure_wall_client():
    Application = get_application_model()
    app = Application.objects.filter(name=WALL_CLIENT_NAME).first()
    if app is not None:
        return app
    return Application.objects.create(
        name=WALL_CLIENT_NAME,
        client_type=Application.CLIENT_PUBLIC,
        authorization_grant_type=Application.GRANT_AUTHORIZATION_CODE,
        redirect_uris=WALL_REDIRECT_URIS,
        skip_authorization=False,
        hash_client_secret=False,
        client_secret="",
    )


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
