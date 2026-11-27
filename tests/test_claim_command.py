import pytest
from django.core.management import call_command
from django.core.management.base import CommandError

from tests.factories import ItemFactory
from trove.models import Activity, Claim
from trove.services.owner import OWNER_USERNAME
from trove.services.recorder import record

pytestmark = pytest.mark.django_db(databases=["default", "data"])


def test_claim_appends_preferred_that_beats_later_derive():
    item = ItemFactory()
    with record("derive", "agent@1", subject=item) as rec:
        rec.claim(item, "base", "E14")
    assert item.current_values()["base"] == "E14"

    call_command("claim", item.id, "base", "E27")

    edit = Activity.objects.get(kind="edit")
    assert edit.agent == OWNER_USERNAME
    preferred = Claim.objects.get(item=item, path="base", rank=Claim.Rank.PREFERRED)
    assert preferred.value == "E27"
    assert preferred.activity_id == edit.id
    assert Claim.objects.filter(item=item, path="base").count() == 2
    assert item.current_values()["base"] == "E27"

    with record("derive", "agent@2", subject=item) as rec:
        rec.claim(item, "base", "E14")
    assert item.current_values()["base"] == "E27"
    assert Claim.objects.filter(item=item, path="base").count() == 3


def test_claim_missing_item_fails():
    with pytest.raises(CommandError, match="does not exist"):
        call_command("claim", "missing", "title", "x")
