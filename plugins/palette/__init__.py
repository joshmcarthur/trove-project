from PIL import Image
from pydantic import BaseModel

from trove.core import Processor
from trove.tasks import Emit, Result


class Colour(BaseModel):
    hex: str


def run(item, ctx):
    values = ctx.values(item)
    sha = values.get("$blob")
    if not sha:
        return Result()
    with Image.open(ctx.blob_path(sha)) as img:
        rgb = img.convert("RGB")
        counted = rgb.getcolors(maxcolors=rgb.size[0] * rgb.size[1]) or []
    emits = []
    seen = set()
    for _n, colour in counted:
        hexv = "#{:02x}{:02x}{:02x}".format(*colour)
        if hexv in seen:
            continue
        seen.add(hexv)
        emits.append(Emit(type="colour", key=hexv, claims={"hex": hexv}))
    return Result(emits=emits)


def setup(core, entry):
    core.add_type("colour", Colour)
    core.add_processor(Processor("palette", 1, "capture.image", ["colour"], run))
