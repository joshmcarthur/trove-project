import { PluginSystem } from "./plugins.ts";
import {
  Event,
  EventId,
  EventQuery,
  EventStorage,
  FileStorage,
  LinkStorage,
  Logger,
  Plugin,
  PluginCapability,
  StorageConfiguration,
} from "./types.ts";

export class StorageManager {
  private eventStorage!: EventStorage;
  private fileStorage?: FileStorage;
  private linkStorage?: LinkStorage;
  private logger: Logger;
  private plugins: PluginSystem;

  constructor(plugins: PluginSystem, logger: Logger) {
    this.plugins = plugins;
    this.logger = logger;
  }

  async initialize(config: StorageConfiguration): Promise<void> {
    // Load storage plugins based on configuration
    if (!config.events) {
      throw new Error("Event storage configuration is required");
    }

    const eventPlugin = await this.loadPlugin(
      config.events.plugin,
      ["storage:events"],
    );
    this.eventStorage = eventPlugin as unknown as EventStorage;
    await this.eventStorage.initialize(config.events.options);

    if (config.files) {
      const filePlugin = await this.loadPlugin(
        config.files.plugin,
        ["storage:files"],
      );
      this.fileStorage = filePlugin as unknown as FileStorage;
      await this.fileStorage.initialize(config.files.options);
    }

    if (config.links && config.links !== "useEventStorage") {
      const linkPlugin = await this.loadPlugin(
        config.links.plugin,
        ["storage:links"],
      );
      this.linkStorage = linkPlugin as unknown as LinkStorage;
      await this.linkStorage.initialize(config.links.options);
    }
  }

  async saveEvent(event: Event): Promise<Event> {
    try {
      // Save files first if any
      if (this.fileStorage && event.files.length > 0) {
        for (const file of event.files) {
          if (!file.id) {
            file.id = await this.fileStorage.saveFile(file);
          }
        }
      }

      // Save event
      const savedEvent = await this.eventStorage.saveEvent(event);

      // Save links if using separate link storage
      if (this.linkStorage && event.links.length > 0) {
        for (const link of event.links) {
          await this.linkStorage.saveLink(event.id, link);
        }
      }

      return savedEvent;
    } catch (error) {
      this.logger.error("Error saving event:", error);
      throw error;
    }
  }

  async getEvent(id: EventId): Promise<Event | null> {
    try {
      const event = await this.eventStorage.getEvent(id);
      if (!event) return null;

      // Load links if using separate link storage
      if (this.linkStorage) {
        event.links = await this.linkStorage.getLinks(id);
      }

      return event;
    } catch (error) {
      this.logger.error(`Error getting event ${id.id}:`, error);
      throw error;
    }
  }

  async queryEvents(query: EventQuery): Promise<Event[]> {
    try {
      return await this.eventStorage.queryEvents(query);
    } catch (error) {
      this.logger.error("Error querying events:", error);
      throw error;
    }
  }

  private loadPlugin(
    pluginName: string,
    requiredCapabilities: PluginCapability[],
  ): Promise<Plugin> {
    const plugin = this.plugins.getPlugin(pluginName, requiredCapabilities);

    if (!plugin) {
      throw new Error(
        `Plugin with required capabilities ${requiredCapabilities} not found: ${pluginName}`,
      );
    }

    return Promise.resolve(plugin);
  }
}
