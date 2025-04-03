---
title: "Welcome"
nav_order: 0
---

# Welcome to Trove

Trove is for holding onto your stuff. Whatever that stuff may be.

Technically speaking, Trove is an event-driven data storage and processing
system built with Deno. The core project provides a flexible plugin architecture
for storing, processing, and serving events with their associated files and
relationships.

## Key Features

- **Plugin-based Architecture**: Extend Trove's functionality through a powerful
  plugin system
- **Flexible Storage**: Choose from multiple storage backends for events and
  files
- **Event-driven Processing**: Process and transform events in real-time
- **File Management**: Attach and manage files alongside your events
- **Event Relationships**: Create rich relationships between events
- **Type Safety**: Built with TypeScript for a great developer experience
- **API Support**: Access your data through a comprehensive API. Use plugins to
  extend the API.

## Getting Started

New to Trove? Start with our
[Quick Start Guide](./getting-started/quick-start.md) to create your first event
processing system.

## Documentation Structure

- **Getting Started**
  - [Installation](./getting-started/installation.md)
  - [Quick Start](./getting-started/quick-start.md)

- **Core Concepts**
  - [Events and Schemas](./concepts/events.md)
  - [Plugins](./concepts/plugins.md)
  - [Storage](./concepts/storage.md)

- **Plugin Development**
  - [Creating Plugins](./plugins/creating-plugins.md)
  - [Storage Plugins](./plugins/storage-plugins.md)
  - [Processing Plugins](./plugins/processing-plugins.md)

## Development

Trove requires Deno v2.0 or later. For development instructions, see our
[GitHub repository](https://github.com/joshmcarthur/trove).

## License

Trove is licensed under the GNU GPLv3 License. See the
[LICENSE](https://github.com/joshmcarthur/trove/blob/main/LICENSE) file for
details.
