---
title: Plugins
nav_order: 4
---

# Plugins

Plugins are the primary way to extend Trove's functionality. They allow you to
add new capabilities without modifying the core system.

## Plugin Types

Trove supports several types of plugins:

- **Event Processing Plugins**: Process and transform events
- **Storage Plugins**: Provide backends for storing events, files, and
  relationships
- **Web Plugins**: Add HTTP endpoints and web functionality

## Documentation

- [Creating Plugins](./plugins/creating-plugins.md) - Learn how to create your
  own plugins
- [Storage Plugins](./plugins/storage-plugins.md) - Details about implementing
  storage backends

## Getting Started

To create a plugin, you'll need to:

1. Define a TypeScript module that exports a plugin definition
2. Implement the required interfaces for your plugin type
3. Register hooks to interact with the system
4. Handle initialization and cleanup

See the [Creating Plugins](./plugins/creating-plugins.md) guide for detailed
instructions.
