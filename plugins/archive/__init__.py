"""Builtin plugin: store an MHTML archive of a public URL."""

from trove.core import Processor

from .run import run


def setup(core, entry):
    core.add_processor(Processor("archive", 1, "capture.link", ["capture.file"], run))
