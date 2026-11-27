import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = os.environ.get("DJANGO_SECRET_KEY", "insecure-dev-key")
DEBUG = True
ALLOWED_HOSTS = ["localhost", "127.0.0.1", "testserver"]

INSTALLED_APPS = [
    "django.contrib.contenttypes",
    "django.contrib.auth",
]

ROOT_URLCONF = "config.urls"

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

USE_TZ = True
DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"
