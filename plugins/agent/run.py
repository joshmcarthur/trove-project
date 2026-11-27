from __future__ import annotations

import base64
import hashlib
import io
import json
import os
import urllib.error
import urllib.request

from PIL import Image

from trove.core import get_core
from trove.models import Blob
from trove.tasks import Emit, Result, TransientError

SKIP_TYPES = frozenset({"colour"})
DEFAULT_MODEL = "gpt-4o-mini"
DEFAULT_ENDPOINT = "https://api.openai.com/v1"

SYSTEM_PROMPT = (
    "Fill the listed types for this item, and skip any that do not apply. "
    "Do not fill colour or capture types. Reply with JSON only."
)


def fillable_types(core) -> list[dict]:
    return [row for row in core.describe() if _prompt_type(row["name"])]


def run(item, ctx):
    core = get_core()
    types = fillable_types(core)
    messages = _messages(item, ctx, types)
    prompt_hash = hashlib.sha256(
        json.dumps(messages, sort_keys=True).encode()
    ).hexdigest()
    model = ctx.options.get("model") or DEFAULT_MODEL
    content, cost, used_model = complete(messages, ctx.options, model)
    emits = _emits(core, _payload(content), {row["name"] for row in types})
    return Result(
        emits=emits,
        cost=cost,
        params={"model": used_model, "prompt_hash": prompt_hash},
    )


def complete(
    messages: list[dict], options: dict, model: str
) -> tuple[str, float | None, str]:
    key = (
        options.get("api_key")
        or os.environ.get("OPENAI_API_KEY")
        or os.environ.get("OPENROUTER_API_KEY")
        or ""
    )
    if not key:
        raise ValueError("agent needs an API key")
    base = (
        options.get("base_url") or options.get("endpoint") or DEFAULT_ENDPOINT
    ).rstrip("/")
    body = json.dumps(
        {
            "model": model,
            "messages": messages,
            "response_format": {"type": "json_object"},
        }
    ).encode()
    request = urllib.request.Request(
        f"{base}/chat/completions",
        data=body,
        headers={
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            payload = json.loads(response.read().decode())
    except urllib.error.HTTPError as exc:
        if exc.code == 429 or exc.code >= 500:
            raise TransientError(
                f"HTTP {exc.code}", retry_after=_retry_after(exc)
            ) from exc
        raise
    except urllib.error.URLError as exc:
        raise TransientError(str(exc)) from exc
    choices = payload.get("choices") or [{}]
    message = (choices[0] or {}).get("message") or {}
    return message.get("content") or "{}", _cost(payload), payload.get("model") or model


def _prompt_type(name: str) -> bool:
    return name not in SKIP_TYPES and not name.startswith("capture.")


def _messages(item, ctx, types: list[dict]) -> list[dict]:
    values = {
        key: value
        for key, value in ctx.values(item).items()
        if not str(key).startswith("$")
    }
    catalog = [
        {
            "name": row["name"],
            "description": row["description"],
            "schema": row["schema"],
        }
        for row in types
    ]
    text = (
        f"Item type: {item.type}\n"
        f"{json.dumps(values, sort_keys=True)}\n\n"
        f"Types:\n{json.dumps(catalog)}\n\n"
        'JSON shape: {"fills":[{"type":"...","key":"...","claims":{...}}]}\n'
        "key is a short id. Keep it the same if this item is filled again."
    )
    user: dict = {"role": "user", "content": text}
    image = _image_part(item, ctx)
    if image is not None:
        user = {
            "role": "user",
            "content": [
                {"type": "text", "text": text},
                image,
            ],
        }
    return [{"role": "system", "content": SYSTEM_PROMPT}, user]


def _image_part(item, ctx) -> dict | None:
    sha = ctx.values(item).get("$blob")
    if not sha:
        return None
    path = ctx.blob_path(sha)
    if not path.exists():
        return None
    blob = Blob.objects.filter(pk=sha).first()
    content_type = blob.content_type if blob else ""
    if content_type and not content_type.startswith("image/"):
        return None
    with Image.open(path) as img:
        buf = io.BytesIO()
        img.convert("RGB").save(buf, "JPEG", quality=85)
    b64 = base64.b64encode(buf.getvalue()).decode("ascii")
    return {
        "type": "image_url",
        "image_url": {"url": f"data:image/jpeg;base64,{b64}"},
    }


def _payload(text: str) -> dict:
    raw = (text or "").strip()
    if raw.startswith("```"):
        lines = raw.split("\n")
        raw = "\n".join(lines[1:])
        if raw.rstrip().endswith("```"):
            raw = raw.rstrip()[:-3]
        raw = raw.strip()
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("agent JSON must be an object")
    return data


def _emits(core, payload: dict, allowed: set[str]) -> list[Emit]:
    fills = payload.get("fills") or []
    if not isinstance(fills, list):
        return []
    emits = []
    for fill in fills:
        if not isinstance(fill, dict):
            continue
        type_name = fill.get("type")
        key = fill.get("key")
        claims = fill.get("claims") or {}
        if not isinstance(type_name, str) or type_name not in allowed:
            continue
        if not key or not isinstance(claims, dict):
            continue
        try:
            core.validate(type_name, claims)
        except Exception:  # noqa: BLE001
            continue
        emits.append(Emit(type=type_name, key=str(key), claims=claims))
    return emits


def _cost(payload: dict) -> float | None:
    usage = payload.get("usage") or {}
    cost = usage.get("cost") if isinstance(usage, dict) else None
    if cost is None:
        return None
    return float(cost)


def _retry_after(exc) -> float | None:
    headers = getattr(exc, "headers", None) or {}
    getter = getattr(headers, "get", None)
    raw = getter("retry-after") if getter else None
    if raw is None and getter:
        raw = getter("Retry-After")
    if raw is None:
        return None
    try:
        return float(raw)
    except (TypeError, ValueError):
        return None
