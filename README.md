# Trove

Trove is an event-driven data storage and processing system built with Deno. It
provides a flexible plugin architecture for storing, processing, and serving
events with their associated files and relationships.

## Features

- Plugin-based architecture
- Flexible storage backends
- Event-driven processing
- File attachment support
- Event linking and relationships
- API support
- Type-safe with TypeScript

## Quick Start

```ts
import { Trove } from "trove/core/mod.ts";

const trove = new Trove({
  plugins: {
    sources: ["./plugins"],
  },
  storage: {
    events: {
      plugin: "storage-json-file",
      options: {
        directory: "./data/events",
      },
    },
    files: {
      plugin: "storage-file-system",
      options: {
        directory: "./data/files",
      },
    },
  },
});

await trove.initialize();

// Create an event
const event = await trove.createEvent({
  schema: "bookmark.created",
  payload: {
    url: "https://example.com",
    title: "Example Website",
  },
});

// Shutdown
await trove.shutdown();
```

## Documentation

Visit our [documentation site](https://trove-project.app) for:

- Getting Started Guide
- Concepts and Architecture
- Plugin Development
- API Reference
- Examples

## Development

Trove requires Deno v2.0 or later.

```bash
# Run tests
deno test

# Check types
deno check

# Format code
deno fmt
```

## License

GNU GPLv3 License - See LICENSE file for details
