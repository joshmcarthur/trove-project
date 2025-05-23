import { assertEquals } from "https://deno.land/std/assert/mod.ts";
import { PluginSystem } from "../plugins.ts";
import { HookSystem } from "../hooks.ts";
import { TestLogger } from "./utils.ts";
import { CoreSystem, Plugin } from "../types.ts";

Deno.test("PluginSystem", async (t) => {
  await t.step("loads and initializes plugins", async () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);
    const core = {} as CoreSystem;
    const plugins = new PluginSystem(core, hooks, logger);

    const testPlugin: Plugin = {
      name: "test-plugin",
      capabilities: [],
      version: "1.0.0",
      initialize: async () => {},
      hooks: {
        "test:hook": () => Promise.resolve(),
      },
    };

    await plugins.loadPlugin(testPlugin);
    assertEquals(plugins.getPlugin("test-plugin"), testPlugin);
  });

  await t.step("prevents duplicate plugin loading", async () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);
    const core = {} as CoreSystem;
    const plugins = new PluginSystem(core, hooks, logger);

    const testPlugin: Plugin = {
      name: "test-plugin",
      version: "1.0.0",
      capabilities: [],
    };

    await plugins.loadPlugin(testPlugin);
    await plugins.loadPlugin(testPlugin);
    assertEquals(logger.logs, [
      {
        args: [],
        level: "info",
        message: "Loading plugin: test-plugin v1.0.0",
      },
      {
        args: [],
        level: "warn",
        message: "Plugin test-plugin is already registered. Skipping loading.",
      },
    ]);
  });

  await t.step("gets plugins by capability", () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);
    const core = {} as CoreSystem;
    const plugins = new PluginSystem(core, hooks, logger);

    const testPlugin: Plugin = {
      name: "test-plugin",
      version: "1.0.0",
      capabilities: [],
    };
    plugins.loadPlugin(testPlugin);

    assertEquals(plugins.getPlugin("test-plugin"), testPlugin);
    assertEquals(
      plugins.getPlugin("test-plugin", ["storage:events"]),
      undefined,
    );
  });

  await t.step("unloads plugins and their hooks", async () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);
    const core = {} as CoreSystem;
    const plugins = new PluginSystem(core, hooks, logger);

    const testPlugin: Plugin = {
      name: "test-plugin",
      version: "1.0.0",
      capabilities: [],
      shutdown: async () => {},
      hooks: {
        "test:hook": () => Promise.resolve(),
      },
    };

    await plugins.loadPlugin(testPlugin);
    await plugins.unloadPlugin("test-plugin");
    assertEquals(plugins.getPlugin("test-plugin"), undefined);
  });
});
