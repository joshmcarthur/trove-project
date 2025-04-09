import { CoreSystem, Logger, Plugin, StorageCapability } from "./types.ts";
import { HookSystem } from "./hooks.ts";
import { PluginLoader } from "./plugin_loader.ts";

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
      this.logger.warn(
        `Plugin ${plugin.name} is already registered. Skipping loading.`,
      );
      return;
    }

    this.plugins.set(plugin.name, plugin);
    this.logger.info(`Loading plugin: ${plugin.name} v${plugin.version}`);

    if (plugin.hooks) {
      for (const [hookName, handler] of Object.entries(plugin.hooks)) {
        this.hooks.registerHook(plugin.name, hookName, handler);
      }
    }

    if (plugin.initialize) {
      try {
        await plugin.initialize(this.core);
        this.logger.debug(`Initialized plugin: ${plugin.name}`);
      } catch (error) {
        this.logger.error(`Failed to initialize plugin ${plugin.name}:`, error);
        await this.unloadPlugin(plugin.name);
        throw new Error(`Initialization failed for plugin ${plugin.name}`);
      }
    }
  }

  async unloadPlugin(pluginName: string): Promise<void> {
    const plugin = this.plugins.get(pluginName);
    if (!plugin) {
      this.logger.debug(
        `Attempted to unload non-existent plugin: ${pluginName}`,
      );
      return;
    }

    if (plugin.shutdown) {
      try {
        await plugin.shutdown();
        this.logger.debug(`Successfully shut down plugin: ${pluginName}`);
      } catch (error) {
        this.logger.error(`Error shutting down plugin ${pluginName}:`, error);
      }
    }

    this.hooks.unregisterPlugin(pluginName);
    this.plugins.delete(pluginName);
    this.logger.info(`Unloaded plugin: ${pluginName}`);
  }

  async loadPlugins(sources: string[]): Promise<void> {
    const loader = new PluginLoader(
      (plugin) => this.loadPlugin(plugin),
      this.logger,
    );
    await loader.loadPluginsFromSources(sources);
    this.logger.info(`Finished processing ${sources.length} plugin source(s).`);
  }

  getPlugin(
    name: string,
    requiredCapabilities?: StorageCapability[],
  ): Plugin | undefined {
    const plugin = this.plugins.get(name);
    if (!plugin) return undefined;

    if (requiredCapabilities) {
      const capabilities = plugin.capabilities || [];
      for (const capability of requiredCapabilities) {
        if (!capabilities.includes(capability)) {
          this.logger.warn(
            `Plugin ${name} does not have required capability: ${capability}`,
          );
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
