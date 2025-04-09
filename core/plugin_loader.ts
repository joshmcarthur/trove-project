import { join, normalize } from "https://deno.land/std@0.220.1/path/mod.ts";
import { isAbsolute } from "https://deno.land/std@0.220.1/path/is_absolute.ts";
import { Logger, Plugin } from "./types.ts";

/**
 * Handles the discovery, loading, and validation of plugins from various sources.
 */
export class PluginLoader {
  private loadPluginCallback: (plugin: Plugin) => Promise<void>;
  private logger: Logger;

  constructor(
    loadPluginCallback: (plugin: Plugin) => Promise<void>,
    logger: Logger,
  ) {
    this.loadPluginCallback = loadPluginCallback;
    this.logger = logger;
  }

  /**
   * Loads plugins from a list of source identifiers (URLs, file paths, directory paths).
   */
  async loadPluginsFromSources(sources: string[]): Promise<void> {
    for (const source of sources) {
      await this.loadFromSource(source);
    }
  }

  /**
   * Loads a plugin or plugins from a single source identifier.
   */
  async loadFromSource(source: string): Promise<void> {
    this.logger.debug(`Processing plugin source: ${source}`);
    try {
      // Check if it's a URL or specifier
      if (
        source.startsWith("http://") || source.startsWith("https://") ||
        source.startsWith("file://") || source.startsWith("jsr:") ||
        source.startsWith("npm:")
      ) {
        this.logger.info(`Loading plugin from URL/Specifier: ${source}`);
        await this._loadAndValidate(source, source);
      } else {
        // Assume it's a local path (file or directory)
        // Path normalization should happen before calling this (e.g., in loadConfig).
        // If relative, resolve against CWD as a fallback.
        const absoluteSourcePath = isAbsolute(source)
          ? source
          : normalize(join(Deno.cwd(), source));

        try {
          const fileInfo = await Deno.stat(absoluteSourcePath);
          if (fileInfo.isDirectory) {
            await this._loadFromDirectory(absoluteSourcePath);
          } else if (fileInfo.isFile) {
            this.logger.info(`Loading plugin from file: ${absoluteSourcePath}`);
            const importUrl = new URL("file://" + absoluteSourcePath).href;
            await this._loadAndValidate(absoluteSourcePath, importUrl);
          } else {
            this.logger.warn(
              `Plugin source is neither file nor directory: ${absoluteSourcePath}`,
            );
          }
        } catch (error: unknown) {
          if (error instanceof Deno.errors.NotFound) {
            this.logger.warn(`Plugin source not found: ${absoluteSourcePath}`);
          } else {
            this.logger.error(
              `Error accessing plugin source ${absoluteSourcePath}:`,
              error,
            );
          }
          // Don't throw, allow continuing with other sources
        }
      }
    } catch (error) {
      // Catch errors during source processing itself (e.g., invalid URL format)
      this.logger.error(`Failed to process plugin source ${source}:`, error);
      // Continue to next source
    }
  }

  private async _loadFromDirectory(directory: string): Promise<void> {
    this.logger.info("Loading plugins from directory:", directory);
    try {
      for await (const entry of Deno.readDir(directory)) {
        if (
          !entry.isFile ||
          !(entry.name.endsWith(".ts") || entry.name.endsWith(".js"))
        ) {
          continue;
        }
        const absolutePath = join(directory, entry.name);
        const importUrl = new URL("file://" + absolutePath).href;
        // Pass description and importUrl to the validation method
        await this._loadAndValidate(absolutePath, importUrl);
      }
    } catch (error: unknown) {
      if (error instanceof Deno.errors.NotFound) {
        this.logger.warn(`Plugin directory not found: ${directory}`);
      } else {
        this.logger.error(`Error reading plugin directory ${directory}:`, error);
      }
      // Don't throw if a directory is optional/doesn't exist
    }
  }

  private async _loadAndValidate(
    sourceDescription: string,
    importUrl: string,
  ): Promise<void> {
    try {
      const module = await import(importUrl);
      if (
        !module.default ||
        (typeof module.default !== "object" &&
          typeof module.default !== "function")
      ) {
        this.logger.warn(
          `Skipping ${sourceDescription}: no suitable default export found`,
        );
        return;
      }

      // Handle factory function or direct object export
      const pluginInstance = typeof module.default === "function"
        ? module.default() // Assuming factory takes no args for now
        : module.default;

      // Basic validation of the plugin object structure
      if (
        !pluginInstance || typeof pluginInstance !== "object" ||
        !pluginInstance.name || !pluginInstance.version
      ) {
        this.logger.warn(
          `Skipping ${sourceDescription}: Invalid plugin structure in default export`,
        );
        return;
      }

      // Use the callback to load the validated plugin into the PluginSystem
      await this.loadPluginCallback(pluginInstance as Plugin);
    } catch (error) {
      this.logger.error(
        `Error importing or validating plugin from ${sourceDescription} (${importUrl}):`,
        error,
      );
      // Don't re-throw here; allow processing other sources/plugins
    }
  }
}