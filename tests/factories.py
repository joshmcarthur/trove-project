"""Django model factories.

Facts go on `data`. ConfigEntry goes on `default`. Later issues add factories
here rather than a second module.
"""

from datetime import timedelta

from django.utils import timezone
from factory import Faker, Sequence, SubFactory
from factory.django import DjangoModelFactory
from oauth2_provider.models import get_access_token_model, set_token_value

from trove.models import Activity, Blob, Claim, ConfigEntry, Item, Link, SetupCode
from trove.services.clients import ensure_wall_client
from trove.services.owner import ensure_owner
from trove.services.tokens import mint_ingest_token


class ActivityFactory(DjangoModelFactory):
    class Meta:
        model = Activity
        database = "data"

    kind = "ingest"
    agent = "test"
    outcome = Activity.Outcome.OK
    started = Faker("date_time", tzinfo=timezone.get_current_timezone())
    ended = Faker("date_time", tzinfo=timezone.get_current_timezone())


class ItemFactory(DjangoModelFactory):
    class Meta:
        model = Item
        database = "data"

    type = "capture.image"
    activity = SubFactory(ActivityFactory)


class BlobFactory(DjangoModelFactory):
    class Meta:
        model = Blob
        database = "data"

    sha256 = Faker("sha256")
    size = 100
    content_type = "image/png"


class ClaimFactory(DjangoModelFactory):
    class Meta:
        model = Claim
        database = "data"

    item = SubFactory(ItemFactory)
    path = "title"
    value = "untitled"
    rank = Claim.Rank.NORMAL
    activity = SubFactory(ActivityFactory)


class LinkFactory(DjangoModelFactory):
    class Meta:
        model = Link
        database = "data"

    source = SubFactory(ItemFactory)
    target = SubFactory(ItemFactory)
    rel = "fits"
    activity = SubFactory(ActivityFactory)


class ConfigEntryFactory(DjangoModelFactory):
    class Meta:
        model = ConfigEntry
        database = "default"

    domain = Sequence(lambda n: f"plugin{n}")
    enabled = True


class SetupCodeFactory(DjangoModelFactory):
    class Meta:
        model = SetupCode
        database = "default"

    code = Sequence(lambda n: f"ab{n:02d}-cd{n:02d}")


def seed_dev_ingest_token(
    client_name: str = "shortcut:iphone", raw_token: str = "dev-shortcut-token"
) -> str:
    ensure_owner()
    return mint_ingest_token(client_name, raw_token=raw_token)


def seed_dev_wall_token(raw_token: str = "dev-wall-token") -> str:
    owner = ensure_owner()
    token = get_access_token_model()(
        user=owner,
        application=ensure_wall_client(),
        expires=timezone.now() + timedelta(hours=1),
        scope="read:all",
    )
    set_token_value(token, raw_token)
    token.save()
    return raw_token
