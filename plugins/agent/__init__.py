from trove.core import Processor
from trove.tasks import Result


def run(item, ctx):
    values = ctx.values(item)
    title = values.get("title") or ""
    kind = "image" if item.type.endswith(".image") else "file"
    article = "An" if kind == "image" else "A"
    return Result(claims={"summary": f"{article} {kind} titled '{title}'"})


def setup(core, entry):
    core.add_processor(Processor("agent", 1, "capture.*", [], run))
