import {
  Event,
  EventFile,
  EventId,
  EventLink,
  EventQuery,
  EventStorage,
  FileStorage,
  LinkStorage,
  Plugin,
} from "@trove/core/types.ts";
import { MemoryEventStorage } from "./event-storage.ts";
import { MemoryFileStorage } from "./file-storage.ts";
import { MemoryLinkStorage } from "./link-storage.ts";

// Storage instances
const eventStorage = new MemoryEventStorage();
const fileStorage = new MemoryFileStorage();
const linkStorage = new MemoryLinkStorage();

// Export a default object that can be used as a plugin
export default {
  name: "memory-storage",
  version: "1.0.0",
  capabilities: ["storage:events", "storage:files", "storage:links"],

  // Plugin interface
  initialize: async () => {
    // No initialization needed
  },

  // Event storage methods
  saveEvent(event: Event): Promise<Event> {
    return eventStorage.saveEvent(event);
  },

  getEvent(id: EventId): Promise<Event | null> {
    return eventStorage.getEvent(id);
  },

  queryEvents(query: EventQuery): Promise<Event[]> {
    return eventStorage.queryEvents(query);
  },

  // File storage methods
  saveFile(file: EventFile): Promise<string> {
    return fileStorage.saveFile(file);
  },

  getFile(fileId: string): Promise<EventFile | null> {
    return fileStorage.getFile(fileId);
  },

  getFileData(fileId: string): Promise<Uint8Array | string> {
    return fileStorage.getFileData(fileId);
  },

  // Link storage methods
  saveLink(sourceEventId: EventId, link: EventLink): Promise<void> {
    return linkStorage.saveLink(sourceEventId, link);
  },

  getLinks(
    eventId: EventId,
    options?: { type?: string },
  ): Promise<EventLink[]> {
    return linkStorage.getLinks(eventId, options);
  },
} satisfies Plugin & FileStorage & LinkStorage & EventStorage;
