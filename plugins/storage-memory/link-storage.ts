import { EventId, EventLink, LinkStorage } from "@trove/core/types.ts";
export class MemoryLinkStorage implements LinkStorage {
  private links: Map<string, EventLink[]> = new Map();

  async initialize(): Promise<void> {
    // No initialization needed for memory storage
  }

  saveLink(sourceEventId: EventId, link: EventLink): Promise<void> {
    const eventLinks = this.links.get(sourceEventId.id) || [];
    eventLinks.push(link);
    this.links.set(sourceEventId.id, eventLinks);
    return Promise.resolve();
  }

  getLinks(
    eventId: EventId,
    options?: { type?: string },
  ): Promise<EventLink[]> {
    const eventLinks = this.links.get(eventId.id) || [];
    if (options?.type) {
      return Promise.resolve(
        eventLinks.filter((link) => link.type === options.type),
      );
    }
    return Promise.resolve(eventLinks);
  }
}
