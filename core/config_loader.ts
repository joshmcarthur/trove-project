import { dirname, join, isAbsolute, normalize } from "https://deno.land/std@0.220.1/path/mod.ts";
import { CoreConfig, Logger } from "./types.ts"; // Assuming CoreConfig is defined in types.ts

/**
 * Handles loading and normalization of the Trove configuration file.
 */
export class ConfigLoader {
  private logger: Logger;

  constructor(logger: Logger) {
    this.logger = logger;
  }

  /**
   * Loads configuration from a given path or URL.
   * Resolves relative paths for plugins based on the config file's location.
   * @param configPathOrUrl - The file path or HTTP(S) URL of the configuration file.
   * @returns The loaded and normalized CoreConfig object.
   * @throws Error if loading or parsing fails.
   */
  async load(configPathOrUrl: string): Promise<CoreConfig> {
    this.logger.info(`Loading configuration from: ${configPathOrUrl}`);
    let importUrl: string;
    let configDir: string;

    try {
      // Handle HTTP/HTTPS URLs
      if (configPathOrUrl.startsWith("http://") || configPathOrUrl.startsWith("https://")) {
        importUrl = configPathOrUrl;
        // For remote configs, base directory for relative plugin paths is CWD
        // Alternatively, could disallow relative paths in remote configs.
        configDir = Deno.cwd();
        this.logger.debug(`Using current working directory (${configDir}) as base for relative paths in remote config.`);
      } else {
        // Handle local file paths (absolute or relative)
        const absolutePath = isAbsolute(configPathOrUrl)
          ? normalize(configPathOrUrl)
          : normalize(join(Deno.cwd(), configPathOrUrl));

        // Ensure the file exists before trying to import
        try {
            await Deno.stat(absolutePath);
        } catch (statError) {
             if (statError instanceof Deno.errors.NotFound) {
                 throw new Error(`Configuration file not found at: ${absolutePath}`);
             }
             throw statError; // Re-throw other stat errors
        }


        importUrl = new URL("file://" + absolutePath).href;
        configDir = dirname(absolutePath);
        this.logger.debug(`Configuration base directory set to: ${configDir}`);
      }

      // Dynamically import the configuration module
      const configModule = await import(importUrl);
      if (!configModule.default || typeof configModule.default !== 'object') {
        throw new Error(`Configuration file must have a default export that is an object.`);
      }
      const config: Partial<CoreConfig> = configModule.default; // Start with partial

      // --- Normalize Configuration ---
      // Ensure core properties and plugin structure exist
      config.plugins = config?.plugins ?? { sources: [] };
      config.plugins.sources = config?.plugins?.sources ?? [];
      config.plugins.config = config?.plugins?.config ?? {};
      // Add other default/required config sections here as needed

      // Normalize plugin source paths
      config.plugins.sources = config.plugins.sources.map((source: string) => {
        // Keep absolute URLs and specifiers as is
        if (
          source.startsWith("http://") || source.startsWith("https://") ||
          source.startsWith("file://") || source.startsWith("jsr:") ||
          source.startsWith("npm:") || isAbsolute(source)
        ) {
          this.logger.debug(`Plugin source is absolute/specifier, keeping as is: ${source}`);
          return source;
        }
        // Resolve relative paths against the config file's directory
        const resolvedPath = join(configDir, source);
        this.logger.debug(`Resolved relative plugin source "${source}" to "${resolvedPath}"`);
        return resolvedPath;
      });

      // --- TODO: Add more validation/normalization here in the future ---
      // Example: Validate storage path, check required fields, etc.

      this.logger.info(`Configuration loaded successfully.`);
      return config as CoreConfig; // Cast to CoreConfig after normalization

    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error(`Failed to load configuration from ${configPathOrUrl}: ${message}`);
      // Re-throw a more specific error to be caught by the CLI runner
      throw new Error(`Configuration loading failed: ${message}`);
    }
  }
}