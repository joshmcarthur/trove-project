import {
  CoreConfig,
  CoreSystem,
  Event,
  EventCreationOptions,
  EventId,
  EventQuery,
  HookContext,
  Logger,
  Plugin,
} from "./types.ts";
import { HookSystem } from "./hooks.ts";
import { PluginSystem } from "./plugins.ts";
import { StorageManager } from "./storage.ts";

export class Trove implements CoreSystem {
  public readonly config: CoreConfig;
  public readonly logger: Logger;

  private readonly plugins: PluginSystem;
  private hooks: HookSystem;
  private storage: StorageManager;
  private initialized = false;

  constructor(config: CoreConfig) {
    this.config = config;
    this.logger = config.logger || console;

    this.hooks = new HookSystem(this.logger);
    this.plugins = new PluginSystem(this, this.hooks, this.logger);
    this.storage = new StorageManager(this.plugins, this.logger);
  }

  async initialize(): Promise<void> {
    if (this.initialized) {
      throw new Error("Trove is already initialized");
    }

    this.logger.info("Initializing Trove...");

    // Load plugins from configured directories
    for (const dir of this.config.plugins.directories) {
      await this.plugins.loadPluginsFromDirectory(Deno.realPathSync(dir));
    }

    // Initialize storage
    await this.storage.initialize(this.config.storage);

    // Execute initialization hook
    await this.hooks.executeHook("system:initialized", { core: this });

    this.initialized = true;
    this.logger.info("Trove initialized successfully");
  }

  async shutdown(): Promise<void> {
    if (!this.initialized) return;

    this.logger.info("Shutting down Trove...");

    // Execute shutdown hook
    await this.hooks.executeHook("system:shutting-down", { core: this });

    // Unload all plugins
    for (const plugin of this.plugins.getAllPlugins()) {
      await this.plugins.unloadPlugin(plugin.name);
    }

    this.initialized = false;
    this.logger.info("Trove shut down successfully");
  }

  async createEvent(
    schema: string,
    payload: Record<string, unknown>,
    options: EventCreationOptions = {},
  ): Promise<Event> {
    this.ensureInitialized();

    const event: Event = {
      id: { id: crypto.randomUUID() },
      createdAt: new Date().toISOString(),
      producer: options.producer || "core",
      schema: { id: schema, version: "1.0" },
      payload,
      files: options.files || [],
      links: options.links || [],
      metadata: options.metadata,
    };

    // Execute pre-save hook
    await this.hooks.executeHook("event:storing", { core: this, event });

    // Save event
    const savedEvent = await this.storage.saveEvent(event);

    // Execute post-save hook
    await this.hooks.executeHook("event:stored", {
      core: this,
      event: savedEvent,
    });

    return savedEvent;
  }

  getEvent(id: EventId): Promise<Event | null> {
    this.ensureInitialized();
    return this.storage.getEvent(id);
  }

  queryEvents(query: EventQuery): Promise<Event[]> {
    this.ensureInitialized();
    return this.storage.queryEvents(query);
  }

  async registerPlugin(plugin: Plugin): Promise<void> {
    await this.plugins.loadPlugin(plugin);
  }

  getPlugin(name: string): Promise<Plugin | undefined> {
    return Promise.resolve(this.plugins.getPlugin(name));
  }

  executeHook(
    name: string,
    context: Partial<HookContext>,
  ): Promise<unknown[]> {
    return this.hooks.executeHook(name, context);
  }

  private ensureInitialized(): void {
    if (!this.initialized) {
      throw new Error("Trove is not initialized");
    }
  }
}

// Re-export types
export * from "./types.ts";
