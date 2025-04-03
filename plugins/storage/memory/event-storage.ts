import { Event, EventId, EventQuery, EventStorage } from "@trove/core/types.ts";

export class MemoryEventStorage implements EventStorage {
  private events: Map<string, Event> = new Map();

  async initialize(): Promise<void> {
    // No initialization needed for memory storage
  }

  saveEvent(event: Event): Promise<Event> {
    this.events.set(event.id.id, event);

    return Promise.resolve(event);
  }

  getEvent(id: EventId): Promise<Event | null> {
    return Promise.resolve(this.events.get(id.id) || null);
  }

  queryEvents(query: EventQuery): Promise<Event[]> {
    let events = Array.from(this.events.values());

    // Apply filters
    if (query.schema) {
      const schemas = Array.isArray(query.schema)
        ? query.schema
        : [query.schema];
      events = events.filter((event) => schemas.includes(event.schema.id));
    }

    if (query.producer) {
      const producers = Array.isArray(query.producer)
        ? query.producer
        : [query.producer];
      events = events.filter((event) => producers.includes(event.producer));
    }

    if (query.timeRange) {
      if (query.timeRange.start) {
        events = events.filter((event) =>
          event.createdAt >= query.timeRange!.start!
        );
      }
      if (query.timeRange.end) {
        events = events.filter((event) =>
          event.createdAt <= query.timeRange!.end!
        );
      }
    }

    if (query.links) {
      events = events.filter((event) => {
        return query.links!.every((linkQuery) => {
          return event.links.some((link) => {
            if (linkQuery.type && link.type !== linkQuery.type) return false;
            if (
              linkQuery.targetEvent &&
              link.targetEvent.id !== linkQuery.targetEvent.id
            ) return false;
            return true;
          });
        });
      });
    }

    if (query.payload) {
      events = events.filter((event) => {
        return Object.entries(query.payload!).every(([key, value]) => {
          return event.payload[key] === value;
        });
      });
    }

    // Apply sorting
    if (query.sort) {
      events.sort((a, b) => {
        for (const sort of query.sort!) {
          const aValue = a[sort.field as keyof Event];
          const bValue = b[sort.field as keyof Event];
          if (aValue === bValue) continue;
          if (aValue === undefined) return 1;
          if (bValue === undefined) return -1;
          const comparison = aValue < bValue ? -1 : 1;
          return sort.direction === "asc" ? comparison : -comparison;
        }
        return 0;
      });
    }

    // Apply pagination
    if (query.offset !== undefined) {
      events = events.slice(query.offset);
    }
    if (query.limit !== undefined) {
      events = events.slice(0, query.limit);
    }

    return Promise.resolve(events);
  }
}
