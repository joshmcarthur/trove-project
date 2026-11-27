from django.utils import timezone
from django.utils.crypto import constant_time_compare, get_random_string

from trove.models.setup_code import SetupCode
from trove.services.owner import owner_has_passkey

_ALPHABET = "abcdefghijkmnpqrstuvwxyz23456789"


def _new_code():
    raw = get_random_string(8, allowed_chars=_ALPHABET)
    return f"{raw[:4]}-{raw[4:]}"


def normalize_code(value: str) -> str:
    compact = "".join(ch for ch in value.lower() if ch.isalnum())
    if len(compact) != 8:
        return value.strip()
    return f"{compact[:4]}-{compact[4:]}"


def unused_setup_code():
    return SetupCode.objects.filter(used_at__isnull=True).order_by("pk").first()


def issue_setup_code() -> str | None:
    if owner_has_passkey():
        return None
    existing = unused_setup_code()
    if existing is not None:
        return existing.code
    row = SetupCode.objects.create(code=_new_code())
    return row.code


def code_matches(submitted: str) -> bool:
    row = unused_setup_code()
    if row is None:
        return False
    return constant_time_compare(normalize_code(submitted), row.code)


def consume_setup_code() -> None:
    row = unused_setup_code()
    if row is None:
        return
    row.used_at = timezone.now()
    row.save(update_fields=["used_at"])


def announce_setup(stdout) -> None:
    code = issue_setup_code()
    if code is None:
        return
    stdout.write(f"Setup code: {code}")
    stdout.write("Open /setup to register the owner passkey.")
    stdout.write("manage.py setup_code shows this again.")
