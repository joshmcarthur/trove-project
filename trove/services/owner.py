from allauth.mfa.models import Authenticator
from django.contrib.auth import get_user_model

OWNER_USERNAME = "owner"


def get_owner():
    return get_user_model().objects.filter(username=OWNER_USERNAME).first()


def ensure_owner():
    User = get_user_model()
    user, _created = User.objects.get_or_create(
        username=OWNER_USERNAME,
        defaults={"is_staff": True, "is_superuser": True},
    )
    dirty = False
    if user.has_usable_password():
        user.set_unusable_password()
        dirty = True
    if not user.is_staff or not user.is_superuser:
        user.is_staff = True
        user.is_superuser = True
        dirty = True
    if dirty:
        user.save()
    return user


def owner_has_passkey():
    owner = get_owner()
    if owner is None:
        return False
    return Authenticator.objects.filter(
        user=owner, type=Authenticator.Type.WEBAUTHN
    ).exists()
