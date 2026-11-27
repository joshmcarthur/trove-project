from trove.core import Processor

from .run import run


def setup(core, entry):
    core.add_processor(Processor("pdf", 1, "capture.document", ["capture.image"], run))
