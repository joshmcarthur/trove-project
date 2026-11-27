from trove.core import Processor
from trove.tasks import Result


def run(item, ctx):
    values = ctx.values(item)
    title = values.get("title") or ""
    kind = "image" if item.type.endswith(".image") else "file"
    return Result(claims={"summary": f"A {kind} titled '{title}'"})


def setup(core, entry):
    core.add_processor(Processor("agent", 1, "capture.*", [], run))
