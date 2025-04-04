import { parse } from "https://deno.land/std@0.220.1/flags/mod.ts";
import { join } from "https://deno.land/std@0.220.1/path/mod.ts";
import {
  blue,
  gray,
  red,
  yellow,
} from "https://deno.land/std@0.220.1/fmt/colors.ts";
import { CoreConfig, Logger, Trove } from "./mod.ts";

class ColorLogger implements Logger {
  debug(message: string, ...args: unknown[]): void {
    console.debug(gray(`[debug] ${message}`), ...args);
  }

  info(message: string, ...args: unknown[]): void {
    console.info(blue(`[info] ${message}`), ...args);
  }

  warn(message: string, ...args: unknown[]): void {
    console.warn(yellow(`[warn] ${message}`), ...args);
  }

  error(message: string, ...args: unknown[]): void {
    console.error(red(`[error] ${message}`), ...args);
  }
}

export async function loadConfig(configPath: string): Promise<CoreConfig> {
  try {
    const configModule = await import(configPath);
    return configModule.default;
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(
      `Failed to load configuration from ${configPath}: ${message}`,
    );
  }
}

export async function run(args: string[] = Deno.args): Promise<void> {
  const flags = parse(args, {
    string: ["config"],
    default: { config: "trove.config.ts" },
  });

  const logger = new ColorLogger();
  const keepAlive = setInterval(() => {}, 5000);

  try {
    const configPath = join(Deno.cwd(), flags.config);
    const config = await loadConfig(configPath);
    config.logger = logger;

    const trove = new Trove(config);
    await trove.initialize();
    logger.info("Trove is running. Press Ctrl+C to stop.");

    Deno.addSignalListener("SIGINT", () => {
      logger.info("Received SIGINT, shutting down...");
      clearTimeout(keepAlive);
      trove.shutdown();
      Deno.exit(0);
    });
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error(message);
    clearTimeout(keepAlive);
    Deno.exit(1);
  }
}

// Run CLI if this module is executed directly
if (import.meta.main) {
  run();
}
