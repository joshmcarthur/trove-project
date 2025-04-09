import {
  assertEquals,
  assertExists,
  assertStringIncludes,
} from "https://deno.land/std@0.220.1/assert/mod.ts";
import {
  dirname,
  fromFileUrl,
  join,
  normalize,
} from "https://deno.land/std@0.220.1/path/mod.ts";
import { PluginLoader } from "../plugin_loader.ts";
import { Plugin } from "../types.ts";
import { TestLogger } from "./utils.ts";

// Get the directory containing this test file
const testDir = dirname(fromFileUrl(import.meta.url));
const fixturesDir = join(testDir, "fixtures");

// --- Create Dummy Plugin Fixture Files ---
const validPluginPath = join(fixturesDir, "valid_plugin.ts");
try {
  await Deno.writeTextFile(
    validPluginPath,
    `export default { name: "valid-plugin", version: "1.0", capabilities: [] };`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}

const factoryPluginPath = join(fixturesDir, "factory_plugin.ts");
try {
  await Deno.writeTextFile(
    factoryPluginPath,
    `export default () => ({ name: "factory-plugin", version: "1.1", capabilities: [] });`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}

const invalidExportPluginPath = join(fixturesDir, "invalid_export_plugin.ts");
try {
  await Deno.writeTextFile(
    invalidExportPluginPath,
    `export const notDefault = { name: "invalid", version: "1.0" };`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}

const invalidStructurePluginPath = join(
  fixturesDir,
  "invalid_structure_plugin.ts",
);
try {
  await Deno.writeTextFile(
    invalidStructurePluginPath,
    `export default { missing_version: true };`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}

const pluginDir = join(fixturesDir, "plugin_dir");
try {
  await Deno.mkdir(pluginDir, { recursive: true });
  await Deno.writeTextFile(
    join(pluginDir, "plugin_a.ts"),
    `export default { name: "plugin-a", version: "1.0", capabilities: [] };`,
  );
  await Deno.writeTextFile(
    join(pluginDir, "plugin_b.js"), // Test .js extension
    `export default { name: "plugin-b", version: "1.0", capabilities: [] };`,
  );
  await Deno.writeTextFile(
    join(pluginDir, "not_a_plugin.txt"),
    `Ignore me`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}

Deno.test("PluginLoader", async (t) => {
  let logger: TestLogger;
  let loadedPlugins: Plugin[];
  let loader: PluginLoader;

  // Mock callback to collect loaded plugins
  const mockLoadPluginCallback = async (plugin: Plugin) => {
    loadedPlugins.push(plugin);
    await Promise.resolve(); // Simulate async behavior
  };

  // Setup before each step
  const setup = () => {
    logger = new TestLogger();
    loadedPlugins = [];
    loader = new PluginLoader(mockLoadPluginCallback, logger);
  };

  await t.step("loads a valid plugin from a file path", async () => {
    setup();
    await loader.loadPluginsFromSources([validPluginPath]);

    assertEquals(loadedPlugins.length, 1);
    assertEquals(loadedPlugins[0].name, "valid-plugin");
    assertEquals(loadedPlugins[0].version, "1.0");
    assertStringIncludes(logger.logs[0]?.message || "", validPluginPath); // Logged loading
  });

  await t.step(
    "loads a valid plugin from a factory function export",
    async () => {
      setup();
      await loader.loadPluginsFromSources([factoryPluginPath]);

      assertEquals(loadedPlugins.length, 1);
      assertEquals(loadedPlugins[0].name, "factory-plugin");
      assertEquals(loadedPlugins[0].version, "1.1");
    },
  );

  await t.step("loads multiple plugins from a directory", async () => {
    setup();
    await loader.loadPluginsFromSources([pluginDir]); // Pass directory path

    assertEquals(loadedPlugins.length, 2);
    assertExists(loadedPlugins.find((p) => p.name === "plugin-a"));
    assertExists(loadedPlugins.find((p) => p.name === "plugin-b"));
    assertStringIncludes(logger.logs[0]?.message || "", pluginDir); // Logged loading from dir
    // Check that non-plugin files were ignored (no error logs)
    assertEquals(logger.logs.filter((l) => l.level === "error").length, 0);
  });

  await t.step("skips plugin file with no default export", async () => {
    setup();
    await loader.loadPluginsFromSources([invalidExportPluginPath]);
    assertEquals(loadedPlugins.length, 0);
    const warnLog = logger.logs.find((l) => l.level === "warn");
    assertExists(warnLog);
    assertStringIncludes(
      warnLog.message,
      `Skipping ${invalidExportPluginPath}: no suitable default export found`,
    );
  });

  await t.step("skips plugin file with invalid structure", async () => {
    setup();
    await loader.loadPluginsFromSources([invalidStructurePluginPath]);
    assertEquals(loadedPlugins.length, 0);
    const warnLog = logger.logs.find((l) => l.level === "warn");
    assertExists(warnLog);
    assertStringIncludes(
      warnLog.message,
      `Skipping ${invalidStructurePluginPath}: Invalid plugin structure`,
    );
  });

  await t.step("handles non-existent source path gracefully", async () => {
    setup();
    const nonExistentPath = join(fixturesDir, "non_existent_plugin.ts");
    await loader.loadPluginsFromSources([nonExistentPath]);
    assertEquals(loadedPlugins.length, 0); // No plugin loaded
    const warnLog = logger.logs.find((l) => l.level === "warn");
    assertExists(warnLog);
    assertStringIncludes(
      warnLog.message,
      `Plugin source not found: ${normalize(nonExistentPath)}`,
    );
  });

  await t.step(
    "loads plugins from mixed sources (file and directory)",
    async () => {
      setup();
      await loader.loadPluginsFromSources([validPluginPath, pluginDir]);

      assertEquals(loadedPlugins.length, 3); // 1 from file + 2 from dir
      assertExists(loadedPlugins.find((p) => p.name === "valid-plugin"));
      assertExists(loadedPlugins.find((p) => p.name === "plugin-a"));
      assertExists(loadedPlugins.find((p) => p.name === "plugin-b"));
    },
  );
});
