---
title: Hooks
parent: Concepts
order: 2
---

# Hook System

The hook system is Trove's primary mechanism for extending functionality and
enabling plugins to interact with the core system and each other.

## What are Hooks?

Hooks are named events that plugins can listen for and respond to. When a hook
is triggered, all registered handlers for that hook are called in sequence,
allowing multiple plugins to process the same event or request.

## Core Hooks

### Event Lifecycle

- `event:validating` - Before event schema validation
- `event:validated` - After successful schema validation
- `event:received` - When a new event is created
- `event:storing` - Before storing an event
- `event:stored` - After an event is stored

### Schema Lifecycle

- `schema:registering` - Before registering a new schema
- `schema:registered` - After a schema is registered

### HTTP Lifecycle

- `http:request` - For each incoming HTTP request
- `http:response` - Before sending an HTTP response

### System Lifecycle

- `system:initializing` - During system startup
- `system:initialized` - After system is ready
- `system:shutting-down` - Before system shutdown

## Using Hooks

Plugins can register hook handlers in their definition:

```ts
export default {
  name: "my-plugin",
  version: "1.0.0",

  hooks: {
    "event:received": async (context) => {
      const { event, core } = context;
      // Process the event
    },

    "http:request": async (context) => {
      const { request } = context;
      // Handle the request or return null to pass to next handler
    },
  },
};
```

## Hook Context

Each hook receives a context object containing relevant data and utilities:

```ts
interface HookContext {
  // The core system API
  core: CoreSystem;

  // Hook-specific data
  event?: Event; // For event hooks
  request?: Request; // For HTTP hooks
  response?: Response; // For HTTP hooks
  schema?: EventSchema; // For schema hooks

  // Shared state for this processing chain
  state: Map<string, any>;

  // Utilities
  logger: Logger;
}
```

## Hook Priority

Plugins can specify priority for their hooks to control execution order:

```ts
export default {
  name: "my-plugin",
  hooks: {
    "event:received": {
      priority: 100, // Higher numbers run first
      handler: async (context) => {
        // Handle event
      },
    },
  },
};
```

## Creating Custom Hooks

Plugins can register their own hooks for other plugins to use:

```ts
export default {
  name: "my-plugin",

  async initialize(core) {
    // Register a new hook
    await core.registerHook("my-plugin:custom-event", {
      schema: {
        // JSON Schema for hook arguments
      },
    });
  },

  async someFunction() {
    // Trigger the hook
    await this.core.executeHook("my-plugin:custom-event", {
      // Hook arguments
    });
  },
};
```

## Best Practices

1. Use descriptive hook names with namespaces
2. Document hook arguments and expected behavior
3. Handle hook failures gracefully
4. Use priorities when order matters
5. Keep hook handlers focused and efficient
