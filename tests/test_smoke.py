import pytest
from django.db import connections


@pytest.mark.django_db(databases=["default", "data"])
def test_both_databases_are_available():
    connections["default"].ensure_connection()
    connections["data"].ensure_connection()
    assert connections["default"].is_usable()
    assert connections["data"].is_usable()
