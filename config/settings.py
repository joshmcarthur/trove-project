import os
from pathlib import Path

from django.core.exceptions import ImproperlyConfigured

BASE_DIR = Path(__file__).resolve().parent.parent
DATA_DIR = Path(os.environ.get("TROVE_DATA_DIR", BASE_DIR))

DEBUG = os.environ.get("DJANGO_DEBUG", "true").lower() in ("1", "true", "yes")
_secret_key = os.environ.get("DJANGO_SECRET_KEY")
SECRET_KEY_FILE = os.environ.get("TROVE_SECRET_KEY_FILE")
if not _secret_key and os.environ.get("TROVE_DATA_DIR"):
    _secret_path = DATA_DIR / ".secret-key"
    if _secret_path.exists():
        _secret_key = _secret_path.read_text().strip()
        if not _secret_key:
            raise ImproperlyConfigured("The persisted Django secret is empty")
        SECRET_KEY_FILE = str(_secret_path)
if not _secret_key and not DEBUG:
    raise ImproperlyConfigured(
        "DJANGO_SECRET_KEY or a persisted Django secret is required"
    )
SECRET_KEY = _secret_key or "insecure-dev-key"
_extra_hosts = os.environ.get("DJANGO_ALLOWED_HOSTS", "")
ALLOWED_HOSTS = [
    "localhost",
    "127.0.0.1",
    "testserver",
    *[h.strip() for h in _extra_hosts.split(",") if h.strip()],
]

INSTALLED_APPS = [
    "django.contrib.contenttypes",
    "django.contrib.auth",
    "django.contrib.sessions",
    "django.contrib.messages",
    "django.contrib.admin",
    "django.contrib.staticfiles",
    "django.contrib.humanize",
    "allauth",
    "allauth.account",
    "allauth.mfa",
    "oauth2_provider",
    "django_ox",
    "trove",
]

MIDDLEWARE = [
    "django.middleware.security.SecurityMiddleware",
    "django.contrib.sessions.middleware.SessionMiddleware",
    "django.middleware.common.CommonMiddleware",
    "django.middleware.csrf.CsrfViewMiddleware",
    "django.contrib.auth.middleware.AuthenticationMiddleware",
    "django.contrib.messages.middleware.MessageMiddleware",
    "allauth.account.middleware.AccountMiddleware",
]

AUTHENTICATION_BACKENDS = [
    "django.contrib.auth.backends.ModelBackend",
    "allauth.account.auth_backends.AuthenticationBackend",
]

ROOT_URLCONF = "config.urls"

TEMPLATES = [
    {
        "BACKEND": "django.template.backends.django.DjangoTemplates",
        "DIRS": [],
        "APP_DIRS": True,
        "OPTIONS": {
            "context_processors": [
                "django.template.context_processors.request",
                "django.contrib.auth.context_processors.auth",
                "django.contrib.messages.context_processors.messages",
            ],
        },
    },
]

ASGI_APPLICATION = "config.asgi.application"

DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.sqlite3",
        "NAME": DATA_DIR / "config.sqlite3",
    },
    "data": {
        "ENGINE": "django.db.backends.sqlite3",
        "NAME": DATA_DIR / "data.sqlite3",
    },
}
DATABASE_ROUTERS = ["trove.routers.TroveRouter"]

USE_TZ = True
DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"
STATIC_URL = "/static/"

LOGIN_URL = "/accounts/login/"
LOGIN_REDIRECT_URL = "/admin/"
ACCOUNT_SIGNUP_ENABLED = False
ACCOUNT_LOGIN_METHODS = {"username"}
ACCOUNT_EMAIL_VERIFICATION = "none"
MAILERS = {
    "default": {
        "BACKEND": "django.core.mail.backends.console.EmailBackend",
    }
}

MFA_SUPPORTED_TYPES = ["webauthn"]
MFA_PASSKEY_LOGIN_ENABLED = True
MFA_ALLOW_UNVERIFIED_EMAIL = True
MFA_WEBAUTHN_ALLOW_INSECURE_ORIGIN = DEBUG

OAUTH2_PROVIDER = {
    "PKCE_REQUIRED": True,
    "SCOPES": {
        "read:all": "Read the collection",
        "write:capture": "Create captures",
    },
    "DEFAULT_SCOPES": [],
    "ACCESS_TOKEN_EXPIRE_SECONDS": 3600,
    "REFRESH_TOKEN_EXPIRE_SECONDS": 60 * 60 * 24 * 30,
    "ROTATE_REFRESH_TOKEN": True,
}

TASKS = {
    "default": {
        "BACKEND": "django_ox.backend.OxBackend",
        "QUEUES": ["default"],
        "OPTIONS": {
            "MAX_ATTEMPTS": 3,
        },
    }
}

TROVE = {
    "BLOB_DIR": DATA_DIR / "blobs",
    "PLUGIN_DIRS": [BASE_DIR / "plugins"],
    "WORKER_CONCURRENCY": 2,
    "INGEST_MAX_BYTES": 512 * 1024 * 1024,
}
