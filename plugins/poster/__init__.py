from trove.core import Processor

from .run import run


def setup(core, entry):
    core.add_processor(Processor("poster", 1, "capture.video", ["capture.image"], run))
