---
title: Events
parent: Concepts
order: 1
---

# Events

Events are the core data structure in Trove. An event represents something that
happened, along with its associated data, files, and relationships to other
events.

## Event Structure

An event consists of:

- **ID**: A unique identifier and optional version
- **Schema**: The type and version of the event
- **Payload**: The actual data of the event (must conform to the schema)
- **Files**: Any associated files or binary data
- **Links**: Relationships to other events
- **Metadata**: Additional information about the event

Example event:

```json
{
  "id": {
    "id": "evt_123abc",
    "version": 1
  },
  "createdAt": "2024-03-15T10:30:00Z",
  "producer": "web-api",
  "schema": {
    "id": "document.uploaded",
    "version": "1.0"
  },
  "payload": {
    "title": "Annual Report",
    "author": "Jane Smith"
  },
  "files": [
    {
      "id": "file_xyz789",
      "contentType": "application/pdf",
      "filename": "report.pdf",
      "size": 1048576,
      "hash": "sha256:abc123...",
      "isReference": true,
      "data": "s3://mybucket/reports/report.pdf"
    }
  ],
  "links": [
    {
      "type": "parent",
      "targetEvent": {
        "id": "evt_456def"
      }
    }
  ],
  "metadata": {
    "ip": "192.168.1.1",
    "userAgent": "Mozilla/5.0..."
  }
}
```

## Event Schemas

Every event must conform to a registered schema. Schemas are defined using JSON
Schema and help ensure data consistency and validation.

```json
{
  "id": "document.uploaded",
  "version": "1.0",
  "schema": {
    "type": "object",
    "properties": {
      "title": {
        "type": "string",
        "minLength": 1
      },
      "author": {
        "type": "string"
      }
    },
    "required": ["title"]
  }
}
```

## Event Links

Events can be linked to create relationships:

- **Parent/Child**: Hierarchical relationships
- **Reference**: Generic connections between events
- **Custom**: Define your own relationship types

Links help build a graph of related events and enable complex queries and
traversals.

## Files and Attachments

Events can include files or references to files:

- Direct binary data
- URLs
- File system paths
- Cloud storage references

The storage backend determines how files are actually stored and retrieved.

## Event Immutability

Events in Trove are immutable by default. Once created, an event cannot be
modified. If you need to update data:

1. Create a new event with the updated data
2. Link it to the original event
3. Use versioning if supported by your storage backend

This ensures a complete audit trail and enables event sourcing patterns.
