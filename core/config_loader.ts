import {
  dirname,
  isAbsolute,
  join,
  normalize,
} from "https://deno.land/std@0.220.1/path/mod.ts";
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
    const cwd = this.getCurrentWorkingDir();

    try {
      // First check if it's a remote URL
      if (
        configPathOrUrl.startsWith("http://") ||
        configPathOrUrl.startsWith("https://")
      ) {
        importUrl = configPathOrUrl;
        configDir = cwd;
        this.logger.debug(
          `Using shell working directory (${configDir}) as base for relative paths in remote config.`,
        );
      } else {
        // For all local paths (whether the CLI is run from URL, absolute path, or locally),
        // resolve config path relative to shell's CWD
        const resolvedConfigPath = isAbsolute(configPathOrUrl)
          ? normalize(configPathOrUrl)
          : normalize(join(cwd, configPathOrUrl));

        try {
          await Deno.stat(resolvedConfigPath);
        } catch (statError) {
          if (statError instanceof Deno.errors.NotFound) {
            throw new Error(
              `Configuration file not found at: ${resolvedConfigPath}`,
            );
          }
          throw statError;
        }

        importUrl = new URL(`file://${resolvedConfigPath}`).href;
        configDir = dirname(resolvedConfigPath);
        this.logger.debug(`Configuration base directory set to: ${configDir}`);
      }

      // Dynamically import the configuration module
      const configModule = await import(importUrl);
      if (!configModule.default || typeof configModule.default !== "object") {
        throw new Error(
          `Configuration file must have a default export that is an object.`,
        );
      }
      const config: Partial<CoreConfig> = configModule.default;

      // --- Normalize Configuration ---
      // Ensure core properties and plugin structure exist
      config.plugins = config?.plugins ?? { sources: [], config: {} };
      config.plugins.sources = config?.plugins?.sources ?? [];
      config.plugins.config = config?.plugins?.config ?? {};

      // Normalize plugin source paths
      config.plugins.sources = config.plugins.sources.map((source: string) => {
        // Keep absolute URLs and specifiers as is
        if (
          source.startsWith("http://") || source.startsWith("https://") ||
          source.startsWith("file://") || source.startsWith("jsr:") ||
          source.startsWith("npm:") || isAbsolute(source)
        ) {
          this.logger.debug(
            `Plugin source is absolute/specifier, keeping as is: ${source}`,
          );
          return source;
        }
        // Resolve relative paths against the config file's directory
        const resolvedPath = join(configDir, source);
        this.logger.debug(
          `Resolved relative plugin source "${source}" to "${resolvedPath}"`,
        );
        return resolvedPath;
      });

      this.logger.info(`Configuration loaded successfully.`);
      return config as CoreConfig;
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error(
        `Failed to load configuration from ${configPathOrUrl}: ${message}`,
      );
      // Re-throw a more specific error to be caught by the CLI runner
      throw new Error(`Configuration loading failed: ${message}`);
    }
  }

  /**
   * Gets the actual shell working directory, not the script directory
   */
  private getCurrentWorkingDir(): string {
    // PWD is more reliable than Deno.cwd() as it represents the shell's working directory
    return Deno.env.get("PWD") || Deno.cwd();
  }
}
