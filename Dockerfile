ARG DENO_VERSION=2.2.8
ARG DENO_BIN_IMAGE=denoland/deno:bin-${DENO_VERSION}

FROM ${DENO_BIN_IMAGE} AS deno_bin

FROM debian:stable-slim

COPY --from=deno_bin /deno /usr/local/bin/deno

RUN useradd --uid 1990 --home-dir /opt/trove --user-group trove \
  && mkdir /opt/trove/ \
  && chown trove:trove /opt/trove/

# Create app directory
WORKDIR /opt/trove

USER trove

# Cache the dependencies as a layer
COPY deno.lock* .
# Copy the app source
COPY . .
RUN deno cache core/cli.ts



ENTRYPOINT ["/usr/local/bin/deno", "run", "--allow-read", "--allow-write", "--allow-net", "core/cli.ts"]