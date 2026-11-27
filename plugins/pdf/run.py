from io import BytesIO

import pypdfium2 as pdfium

from trove.tasks import Emit, Result

STILL_KEY = "page-1"
RENDER_SCALE = 2


def run(item, ctx):
    values = ctx.values(item)
    sha = values.get("$blob")
    if not sha:
        raise ValueError("document has no blob")
    with pdfium.PdfDocument(ctx.blob_path(sha)) as pdf:
        if len(pdf) < 1:
            raise ValueError("PDF has no pages")
        text = _embedded_text(pdf)
        png, width, height = _page_png(pdf[0])
    claims = {"text": text} if text else {}
    return Result(
        claims=claims,
        emits=[
            Emit(
                type="capture.image",
                key=STILL_KEY,
                claims={"width": width, "height": height},
                blob=png,
                content_type="image/png",
            )
        ],
    )


def _page_png(page):
    try:
        bitmap = page.render(scale=RENDER_SCALE)
        try:
            image = bitmap.to_pil()
        finally:
            bitmap.close()
        buf = BytesIO()
        image.save(buf, "PNG")
        return buf.getvalue(), image.width, image.height
    finally:
        page.close()


def _embedded_text(pdf) -> str:
    parts = []
    for index in range(len(pdf)):
        page = pdf[index]
        try:
            textpage = page.get_textpage()
            try:
                chunk = textpage.get_text_range().strip()
            finally:
                textpage.close()
        finally:
            page.close()
        if chunk:
            parts.append(chunk)
    return "\n".join(parts)
