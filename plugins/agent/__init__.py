from trove.core import REGISTRY, Processor

from .run import run
from .types import Bulb, Link, ReferenceShot


def setup(core, entry):
    core.add_type("bulb", Bulb)
    core.add_type("reference_shot", ReferenceShot)
    core.add_type("link", Link)
    core.add_processor(Processor("agent", 1, "capture.*", REGISTRY, run))
