import { parse } from "https://deno.land/std@0.220.1/flags/mod.ts";
import {
  blue,
  gray,
  red,
  yellow,
} from "https://deno.land/std@0.220.1/fmt/colors.ts";
import { CoreConfig, Logger, Trove } from "./mod.ts";
import { ConfigLoader } from "./config_loader.ts";

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

export async function run(args: string[] = Deno.args): Promise<void> {
  const flags = parse(args, {
    string: ["config"],
    alias: { c: "config" },
    default: { config: "trove.config.ts" },
  });

  const logger = new ColorLogger();
  const keepAlive = setInterval(() => {}, 5000);

  try {
    const configLoader = new ConfigLoader(logger);
    const config: CoreConfig = await configLoader.load(flags.config);

    config.logger = logger;

    const trove = new Trove(config);
    await trove.initialize();
    logger.info("Trove initialized successfully. Press Ctrl+C to stop.");

    Deno.addSignalListener("SIGINT", async () => {
      logger.info("Received SIGINT, shutting down...");
      clearInterval(keepAlive);
      try {
        await trove.shutdown();
        logger.info("Trove shutdown complete.");
        Deno.exit(0);
      } catch (shutdownError) {
        logger.error("Error during shutdown:", shutdownError);
        Deno.exit(1);
      }
    });

  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error(`Fatal error during startup: ${message}`);
    clearInterval(keepAlive);
    Deno.exit(1);
  }
}

if (import.meta.main) {
  run();
}
