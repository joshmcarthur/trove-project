FROM python:3.13-slim-bookworm

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    TROVE_DATA_DIR=/data \
    DJANGO_DEBUG=false

WORKDIR /app

COPY pyproject.toml uv.lock README.md ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev

RUN apt-get update \
    && apt-get install -y --no-install-recommends libatomic1 \
    && /app/.venv/bin/playwright install --with-deps chromium-headless-shell \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 1990 trove \
    && useradd --uid 1990 --gid trove --create-home trove \
    && mkdir -p /data \
    && chown -R trove:trove /data /ms-playwright

COPY --chown=trove:trove manage.py docker-entrypoint.sh LICENSE ./
COPY --chown=trove:trove config/ ./config/
COPY --chown=trove:trove trove/ ./trove/
COPY --chown=trove:trove plugins/ ./plugins/

ENV PATH="/app/.venv/bin:$PATH"

USER trove
EXPOSE 8123
VOLUME ["/data"]

ENTRYPOINT ["./docker-entrypoint.sh"]
