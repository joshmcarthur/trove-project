from io import BytesIO

import av

from trove.tasks import Emit, Result

POSTER_KEY = "poster"


def run(item, ctx):
    values = ctx.values(item)
    sha = values.get("$blob")
    if not sha:
        raise ValueError("video has no blob")
    png, width, height = _poster_png(ctx.blob_path(sha))
    return Result(
        emits=[
            Emit(
                type="capture.image",
                key=POSTER_KEY,
                claims={"width": width, "height": height},
                blob=png,
                content_type="image/png",
            )
        ],
    )


def _poster_png(path):
    with av.open(str(path)) as container:
        streams = container.streams.video
        if not streams:
            raise ValueError("video has no video stream")
        for frame in container.decode(streams[0]):
            image = frame.to_image()
            if image.width < 1 or image.height < 1:
                raise ValueError("poster frame has no size")
            buf = BytesIO()
            image.save(buf, "PNG")
            return buf.getvalue(), image.width, image.height
    raise ValueError("video has no frames")
