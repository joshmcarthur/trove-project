"""Django model factories.

Facts go on `data`. ConfigEntry goes on `default`. Later issues add factories
here rather than a second module.
"""

from django.utils import timezone
from factory import Faker, Sequence, SubFactory
from factory.django import DjangoModelFactory

from trove.models import Activity, Blob, Claim, ConfigEntry, Item, Link


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
