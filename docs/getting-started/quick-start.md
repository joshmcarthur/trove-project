---
title: Quick Start
nav_order: 2
---

# Quick Start Guide

This guide will help you get started with Trove by creating a simple event
processing system.

## Basic Setup

Create a new directory for your project:

```bash
mkdir my-trove-project
cd my-trove-project
```

Create a `main.ts` file:

```ts
import { Trove } from "https://deno.land/x/trove/core/mod.ts";

const trove = new Trove({
  plugins: {
    directories: ["./plugins"],
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

// Create an event schema
await trove.registerSchema({
  id: "note.created",
  version: "1.0",
  schema: {
    type: "object",
    properties: {
      title: { type: "string" },
      content: { type: "string" },
    },
    required: ["title", "content"],
  },
});

// Create an event
const event = await trove.createEvent({
  schema: "note.created",
  payload: {
    title: "My First Note",
    content: "Hello, Trove!",
  },
});

console.log("Created event:", event.id);

await trove.shutdown();
```

Run your application:

```bash
deno run --allow-read --allow-write main.ts
```

## Adding a Plugin

Create a simple plugin that processes note events. Create
`plugins/note-processor.ts`:

```ts
export default {
  name: "note-processor",
  version: "1.0.0",

  hooks: {
    "event:received": async (context) => {
      const { event } = context;

      if (event.schema.id === "note.created") {
        console.log(`Processing note: ${event.payload.title}`);

        // Create a processed event
        await context.core.createEvent({
          schema: "note.processed",
          payload: {
            originalId: event.id.id,
            wordCount: event.payload.content.split(/\s+/).length,
          },
          links: [{
            type: "parent",
            targetEvent: event.id,
          }],
        });
      }
    },
  },
};
```

## Next Steps

- Learn more about [Events and Schemas](../concepts/events.md)
- Explore [Plugin Development](../plugins/creating-plugins.md)
- Set up [Storage Backends](../plugins/storage-plugins.md)
