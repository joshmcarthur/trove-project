import { CoreSystem, Logger, Plugin, StorageCapability } from "./types.ts";
import { HookSystem } from "./hooks.ts";

export class PluginSystem {
  private plugins: Map<string, Plugin> = new Map();
  private hooks: HookSystem;
  private core: CoreSystem;
  private logger: Logger;

  constructor(core: CoreSystem, hooks: HookSystem, logger: Logger) {
    this.core = core;
    this.hooks = hooks;
    this.logger = logger;
  }

  async loadPlugin(plugin: Plugin): Promise<void> {
    if (this.plugins.has(plugin.name)) {
      throw new Error(`Plugin ${plugin.name} is already registered`);
    }

    this.plugins.set(plugin.name, plugin);
    this.logger.info(`Loading plugin: ${plugin.name} v${plugin.version}`);

    // Register hooks
    if (plugin.hooks) {
      for (const [hookName, handler] of Object.entries(plugin.hooks)) {
        this.hooks.registerHook(plugin.name, hookName, handler);
      }
    }

    // Initialize plugin
    if (plugin.initialize) {
      try {
        await plugin.initialize(this.core);
        this.logger.debug(`Initialized plugin: ${plugin.name}`);
      } catch (error) {
        this.logger.error(`Failed to initialize plugin ${plugin.name}:`, error);
        await this.unloadPlugin(plugin.name);
        throw error;
      }
    }
  }

  async unloadPlugin(pluginName: string): Promise<void> {
    const plugin = this.plugins.get(pluginName);
    if (!plugin) return;

    if (plugin.shutdown) {
      try {
        await plugin.shutdown();
      } catch (error) {
        this.logger.error(`Error shutting down plugin ${pluginName}:`, error);
      }
    }

    this.hooks.unregisterPlugin(pluginName);
    this.plugins.delete(pluginName);
    this.logger.info(`Unloaded plugin: ${pluginName}`);
  }

  async loadPluginsFromDirectory(directory: string): Promise<void> {
    try {
      for await (const entry of Deno.readDir(directory)) {
        if (!entry.isFile || !entry.name.endsWith(".ts")) continue;

        const module = await import(`file://${directory}/${entry.name}`);
        if (!module.default || typeof module.default !== "object") {
          this.logger.warn(`Skipping ${entry.name}: no default export`);
          continue;
        }

        await this.loadPlugin(module.default);
      }
    } catch (error) {
      this.logger.error(`Error loading plugins from ${directory}:`, error);
      throw error;
    }
  }

  getPlugin(
    name: string,
    requiredCapabilities?: StorageCapability[],
  ): Plugin | undefined {
    const plugin = this.plugins.get(name);
    if (!plugin) return undefined;

    if (requiredCapabilities) {
      for (const capability of requiredCapabilities) {
        if (!plugin.capabilities.includes(capability)) {
          return undefined;
        }
      }
    }

    return plugin;
  }

  getAllPlugins(): Plugin[] {
    return Array.from(this.plugins.values());
  }
}
