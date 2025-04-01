---
title: Storage Plugins
order: 2
---

# Storage Plugins

Storage plugins provide backends for storing events, files, and relationships in
Trove.

## Storage Types

### Event Storage

Stores event data and metadata:

```ts
interface EventStoragePlugin extends StoragePlugin {
  type: "event";

  saveEvent(event: Event): Promise<Event>;
  getEvent(id: EventId): Promise<Event | null>;
  queryEvents(query: EventQuery): Promise<Event[]>;
}
```

### File Storage

Handles binary and text file data:

```ts
interface FileStoragePlugin extends StoragePlugin {
  type: "file";

  saveFile(file: EventFile): Promise<string>;
  getFile(fileId: string): Promise<EventFile | null>;
  getFileData(fileId: string): Promise<Uint8Array | string>;
}
```

### Link Storage

Manages relationships between events:

```ts
interface LinkStoragePlugin extends StoragePlugin {
  type: "link";

  saveLink(sourceEventId: EventId, link: EventLink): Promise<void>;
  getLinks(eventId: EventId, options?: { type?: string }): Promise<EventLink[]>;
  getLinkedEvents(
    eventId: EventId,
    options?: { type?: string },
  ): Promise<Event[]>;
}
```

## Official Storage Plugins

### JSON File Storage

Simple file-based storage for development:

```ts
import { Trove } from "trove/core/mod.ts";

const trove = new Trove({
  storage: {
    events: {
      plugin: "storage-json-file",
      options: {
        directory: "./data/events",
      },
    },
  },
});
```

### SQLite Storage

Embedded database storage:

```ts
{
  storage: {
    events: {
      plugin: "storage-sqlite",
      options: {
        path: "./data/events.db"
      }
    }
  }
}
```

### S3 File Storage

Cloud storage for files:

```ts
{
  storage: {
    files: {
      plugin: "storage-s3",
      options: {
        region: "us-west-2",
        bucket: "my-events",
        accessKeyId: "...",
        secretAccessKey: "..."
      }
    }
  }
}
```

## Creating Storage Plugins

### Basic Structure

```ts
export default {
  name: "my-storage",
  type: "event",

  async initialize(core) {
    // Setup storage
    this.connection = await createConnection(core.config);
  },

  async shutdown() {
    // Cleanup
    await this.connection.close();
  },

  // Implement storage interface
  async saveEvent(event) {
    // Store event
    return event;
  },

  async getEvent(id) {
    // Retrieve event
    return event;
  },

  async queryEvents(query) {
    // Search events
    return events;
  },
};
```

### Query Support

Storage plugins should support flexible queries:

```ts
interface EventQuery {
  schema?: string | string[];
  producer?: string | string[];
  timeRange?: {
    start?: string;
    end?: string;
  };
  links?: {
    type?: string;
    targetEvent?: EventId;
  }[];
  payload?: Record<string, any>;
  limit?: number;
  offset?: number;
  sort?: {
    field: string;
    direction: "asc" | "desc";
  }[];
}
```

### Transactions

Optional transaction support:

```ts
interface TransactionalStoragePlugin extends StoragePlugin {
  beginTransaction(): Promise<void>;
  commitTransaction(): Promise<void>;
  rollbackTransaction(): Promise<void>;
}
```

## Best Practices

1. Implement proper error handling
2. Use connection pooling when appropriate
3. Support efficient querying
4. Implement proper cleanup
5. Consider implementing transactions
6. Document performance characteristics
7. Handle concurrent access
8. Implement proper logging
