---
title: Concepts
nav_order: 3
---

# Core Concepts

Trove is built around a few key concepts that work together to provide a
flexible and powerful system for storing and processing your data.

## Events

Events are the fundamental building blocks of Trove. They represent something
that happened, along with its associated data, files, and relationships to other
events.

Key aspects of events:

- **Immutable**: Once created, events cannot be modified
- **Schema-based**: Every event must conform to a defined schema
- **File attachments**: Can include files or references to files
- **Relationships**: Can be linked to other events to create a graph of data

Learn more about [Events](./concepts/events.md)

## Plugins

Trove's plugin system allows you to extend and customize its functionality.
Plugins can:

- Process events
- Provide storage backends
- Add web endpoints
- Register new event types
- Extend core functionality

Plugins are the primary way to add new capabilities to Trove without modifying
the core system.

Learn more about [Plugins](./concepts/plugins.md)

## Hooks

The hook system is how plugins interact with Trove and each other. Hooks are
named events that plugins can listen for and respond to, enabling:

- Event processing
- HTTP request handling
- System lifecycle management
- Custom plugin interactions

Hooks provide a flexible way to extend Trove's behavior at key points in its
operation.

Learn more about [Hooks](./concepts/hooks.md)

## How It All Works Together

1. **Events** represent your data and its relationships
2. **Plugins** extend Trove's capabilities
3. **Hooks** enable plugins to interact with the system

For example:

- A storage plugin might listen for `event:received` hooks to store events
- A processing plugin might listen for `event:stored` hooks to transform data
- A web plugin might handle `http:request` hooks to serve your data

This architecture allows you to build complex systems while keeping the core
simple and focused.
