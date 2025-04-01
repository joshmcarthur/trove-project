---
title: Creating Plugins
order: 1
---

# Creating Plugins

This guide will walk you through creating plugins for Trove.

## Plugin Structure

A Trove plugin is a TypeScript module that exports a plugin definition:

```ts
import type { CoreSystem, Plugin } from "trove/core/types.ts";

export default {
  // Required: Plugin identification
  name: "my-plugin",
  version: "1.0.0",

  // Optional: Plugin initialization
  async initialize(core: CoreSystem) {
    // Setup code
  },

  // Optional: Cleanup
  async shutdown() {
    // Cleanup code
  },

  // Optional: Hook handlers
  hooks: {
    "event:received": async (context) => {
      // Process events
    },
  },
} satisfies Plugin;
```

## Plugin Types

### Event Processing Plugins

Process events and optionally create new ones:

```ts
export default {
  name: "image-processor",
  version: "1.0.0",

  async initialize(core) {
    // Register our schema
    await core.registerSchema({
      id: "image.processed",
      version: "1.0",
      schema: {
        type: "object",
        properties: {
          originalId: { type: "string" },
          width: { type: "number" },
          height: { type: "number" },
        },
      },
    });
  },

  hooks: {
    "event:received": async ({ event, core }) => {
      if (event.schema.id !== "image.uploaded") return;

      // Process image
      const imageFile = event.files[0];
      const dimensions = await getImageDimensions(imageFile);

      // Create processed event
      await core.createEvent({
        schema: "image.processed",
        payload: {
          originalId: event.id.id,
          ...dimensions,
        },
        links: [{
          type: "parent",
          targetEvent: event.id,
        }],
      });
    },
  },
};
```

### Storage Plugins

Implement storage backends:

```ts
export default {
  name: "my-storage",
  version: "1.0.0",
  type: "storage",

  async initialize(core) {
    // Setup storage connection
  },

  // Implement storage interface
  async saveEvent(event) {
    // Store event
  },

  async getEvent(id) {
    // Retrieve event
  },

  async queryEvents(query) {
    // Search events
  },
};
```

### Web Plugins

Add HTTP endpoints:

```ts
export default {
  name: "my-api",
  version: "1.0.0",

  hooks: {
    "http:request": async ({ request }) => {
      if (request.url.endsWith("/api/my-endpoint")) {
        return new Response("Hello from my plugin!", {
          status: 200,
          headers: {
            "Content-Type": "text/plain",
          },
        });
      }

      // Return null to pass to next handler
      return null;
    },
  },
};
```

## Plugin Configuration

Plugins can accept configuration through the core config:

```ts
// In your Trove configuration
{
  plugins: {
    "my-plugin": {
      option1: "value1",
      option2: "value2"
    }
  }
}

// In your plugin
export default {
  name: "my-plugin",

  async initialize(core) {
    const config = core.config.plugins[this.name];
    this.option1 = config.option1;
  }
};
```

## Testing Plugins

Create test files alongside your plugins:

```ts
// my-plugin.test.ts
import { assertEquals } from "https://deno.land/std/testing/asserts.ts";
import { createTestCore } from "trove/testing/mod.ts";
import myPlugin from "./my-plugin.ts";

Deno.test("my plugin processes events correctly", async () => {
  const core = await createTestCore();
  await core.loadPlugin(myPlugin);

  const event = await core.createEvent({
    schema: "test.event",
    payload: {/* ... */},
  });

  // Assert expected behavior
});
```

## Publishing Plugins

1. Create a repository for your plugin
2. Add documentation
3. Add tests
4. Publish to a Deno module registry or host on GitHub
5. Add to Trove's plugin directory (optional)

## Best Practices

1. Use TypeScript for type safety
2. Handle errors gracefully
3. Clean up resources in shutdown
4. Document hook usage
5. Follow semantic versioning
6. Include tests
7. Provide clear documentation
