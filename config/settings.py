import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = os.environ.get("DJANGO_SECRET_KEY", "insecure-dev-key")
DEBUG = True
ALLOWED_HOSTS = ["localhost", "127.0.0.1", "testserver"]

INSTALLED_APPS = [
    "django.contrib.contenttypes",
    "django.contrib.auth",
    "django.contrib.sessions",
    "django.contrib.messages",
    "django.contrib.admin",
    "django.contrib.staticfiles",
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
        "NAME": BASE_DIR / "config.sqlite3",
    },
    "data": {
        "ENGINE": "django.db.backends.sqlite3",
        "NAME": BASE_DIR / "data.sqlite3",
    },
}
DATABASE_ROUTERS = ["trove.routers.TroveRouter"]

USE_TZ = True
DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"
STATIC_URL = "static/"

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
    "BLOB_DIR": BASE_DIR / "blobs",
    "PLUGIN_DIRS": [BASE_DIR / "plugins"],
    "WORKER_CONCURRENCY": 2,
    "INGEST_TOKENS": {"dev-shortcut-token": "shortcut:iphone"},
}
