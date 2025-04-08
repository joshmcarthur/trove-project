FROM denoland/deno:debian

# Create app directory
WORKDIR /app

# Cache the dependencies as a layer
COPY deno.lock* .
# Copy the app source
COPY . .
RUN deno cache core/cli.ts

# Compile the main app
RUN deno compile --allow-read --allow-write --allow-net --output trove core/cli.ts

# The binary runs with the correct permissions
USER deno

ENTRYPOINT ["/app/trove"]