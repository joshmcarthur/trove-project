# Plugin System

Trove's plugin system is designed to be flexible and extensible. Plugins can:

- Process events
- Provide storage backends
- Add web endpoints
- Register new event types
- Extend the core functionality

## Plugin Structure

A basic plugin consists of a TypeScript module that exports a plugin definition:

```ts
export default {
  name: "my-plugin",
  version: "1.0.0",

  // Optional: Register hooks
  hooks: {
    "event:received": async (context) => {
      // Process event
    },
    "http:request": async (request) => {
      // Handle HTTP request
    },
  },

  // Optional: Initialize plugin
  async initialize(core) {
    // Setup plugin
  },

  // Optional: Cleanup
  async shutdown() {
    // Cleanup
  },
};
```

## Hook System

Plugins interact with the core system and each other through hooks. Hooks are
named events that plugins can register handlers for. When a hook is triggered,
all registered handlers are called in order of priority.

Common hooks include:

- `event:received` - Called when a new event is created
- `event:stored` - Called after an event is stored
- `http:request` - Called for each HTTP request
- `schema:register` - Called when a new schema is registered

## Storage Plugins

Storage plugins provide backends for storing events, files, and relationships. A
storage plugin must implement one or more storage interfaces:

- `EventStoragePlugin` - For storing event data
- `FileStoragePlugin` - For storing file data
- `LinkStoragePlugin` - For storing relationships

See [Storage Plugins](./storage-plugins.md) for more details.

## Web Plugins

Web plugins can extend Trove's HTTP interface by:

- Adding new routes
- Processing requests
- Serving static files
- Implementing authentication
- Providing WebSocket endpoints

## Creating Plugins

See our [Plugin Development Guide](./creating-plugins.md) for detailed
information on creating plugins.
