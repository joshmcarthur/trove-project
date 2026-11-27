import pytest

from tests.factories import ItemFactory
from trove.core import Processor, get_core
from trove.models import Item, Link
from trove.tasks import Emit, Result, ResultLink
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])


def _fitter(version, *, include_fits=True):
    def run(item, ctx):
        bulb = Emit("bulb", "e27", {"base": "E27", "shape": "A60"})
        links = []
        if include_fits:
            links.append(ResultLink(source=bulb, target=item, rel="fits"))
        return Result(emits=[bulb], links=links)

    return Processor("fitter", version, "capture.*", ["bulb"], run)


def test_processor_result_writes_fits_without_duplicating_or_dropping_derived_from():
    get_core()
    lamp = ItemFactory()

    execute_derive(_fitter(1), lamp.id)
    bulb = Item.objects.get(type="bulb")

    def live(rel):
        return Link.objects.live().filter(source=bulb, target=lamp, rel=rel)

    assert live("derived_from").count() == 1
    assert live("fits").count() == 1

    execute_derive(_fitter(1), lamp.id)
    assert live("fits").count() == 1

    execute_derive(_fitter(2), lamp.id)
    assert live("fits").count() == 1
    assert Link.objects.filter(source=bulb, target=lamp, rel="fits").count() == 1

    execute_derive(_fitter(3, include_fits=False), lamp.id)
    assert live("fits").count() == 0
    assert live("derived_from").count() == 1
    assert (
        Link.objects.filter(
            source=bulb, target=lamp, rel="fits", retracts__isnull=False
        ).count()
        == 1
    )
