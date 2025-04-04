import { CoreConfig, Logger } from "../types.ts";
import { Trove } from "@trove/core/mod.ts";
import MemoryPlugin from "@trove/plugins/storage-memory.ts";

export class TestLogger implements Logger {
  public logs: Array<{ level: string; message: string; args: unknown[] }> = [];

  debug(message: string, ...args: unknown[]): void {
    this.logs.push({ level: "debug", message, args });
  }

  info(message: string, ...args: unknown[]): void {
    this.logs.push({ level: "info", message, args });
  }

  warn(message: string, ...args: unknown[]): void {
    this.logs.push({ level: "warn", message, args });
  }

  error(message: string, ...args: unknown[]): void {
    this.logs.push({ level: "error", message, args });
  }

  clear(): void {
    this.logs = [];
  }
}

export async function createTestCore(
  overrides: Partial<CoreConfig> = {},
): Promise<Trove> {
  const config = createTestConfig(overrides);
  const trove = new Trove(config);
  trove.registerPlugin(MemoryPlugin);
  await trove.initialize();

  return trove;
}

export function createTestConfig(
  overrides: Partial<CoreConfig> = {},
): CoreConfig {
  return {
    storage: {
      events: {
        plugin: "memory-storage",
        options: {},
      },
      files: {
        plugin: "memory-storage",
        options: {},
      },
      links: "useEventStorage",
    },
    plugins: {
      directories: [],
    },
    logger: new TestLogger(),
    ...overrides,
  };
}
