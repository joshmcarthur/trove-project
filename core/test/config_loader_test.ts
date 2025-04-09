import {
  assertEquals,
  assertRejects,
  assertStringIncludes,
} from "https://deno.land/std@0.220.1/assert/mod.ts";
import {
  dirname,
  fromFileUrl,
  join,
  normalize,
} from "https://deno.land/std@0.220.1/path/mod.ts";
import { ConfigLoader } from "../config_loader.ts";
import { TestLogger } from "./utils.ts"; // Assuming you have a TestLogger utility

// Get the directory containing this test file
const testDir = dirname(fromFileUrl(import.meta.url));
const fixturesDir = join(testDir, "fixtures");

// Create a dummy fixture config file if it doesn't exist
// In a real scenario, this should be part of your test setup
const validConfigPath = join(fixturesDir, "valid.config.ts");
try {
  await Deno.writeTextFile(
    validConfigPath,
    `
    export default {
      storage: { events: { plugin: "memory-storage", options: {} } },
      plugins: {
        sources: [
          "./relative_plugin.ts", // Relative path
          "https://example.com/remote_plugin.ts", // Absolute URL
          "/absolute_plugin.ts" // Absolute path
        ],
        config: { /* plugin specific config */ }
      }
    };
  `,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}
const invalidExportConfigPath = join(fixturesDir, "invalid_export.config.ts");
try {
  await Deno.writeTextFile(
    invalidExportConfigPath,
    `export const notDefault = {};`,
  );
} catch (e) {
  if (!(e instanceof Deno.errors.AlreadyExists)) throw e;
}


Deno.test("ConfigLoader", async (t) => {
  let logger: TestLogger;
  let configLoader: ConfigLoader;

  // Setup before each test step
  const setup = () => {
    logger = new TestLogger();
    configLoader = new ConfigLoader(logger);
  };

  await t.step("loads valid configuration and normalizes paths", async () => {
    setup();
    const config = await configLoader.load(validConfigPath);

    assertEquals(typeof config, "object");
    assertEquals(config.storage?.events?.plugin, "memory-storage");
    assertEquals(config.plugins?.sources?.length, 3);

    // Check normalization of plugin sources based on config file location
    const expectedBaseDir = dirname(validConfigPath);
    assertEquals(config.plugins.sources[0], join(expectedBaseDir, "relative_plugin.ts"));
    assertEquals(config.plugins.sources[1], "https://example.com/remote_plugin.ts");
    assertEquals(config.plugins.sources[2], "/absolute_plugin.ts"); // Absolute paths remain unchanged

    // Check logger output (optional)
    assertStringIncludes(logger.logs[0]?.message || "", validConfigPath);
    assertStringIncludes(logger.logs.at(-1)?.message || "", "Configuration loaded successfully.");
  });

  await t.step("throws error for non-existent configuration file", async () => {
    setup();
    const configPath = join(fixturesDir, "nonexistent.config.ts");
    await assertRejects(
      () => configLoader.load(configPath),
      Error,
      `Configuration loading failed: Configuration file not found at: ${normalize(configPath)}`,
    );
  });

   await t.step("throws error for configuration without default export", async () => {
      setup();
      await assertRejects(
          () => configLoader.load(invalidExportConfigPath),
          Error,
          "Configuration loading failed: Configuration file must have a default export that is an object."
      );
  });

  // Add more tests:
  // - Loading from HTTP URL (might require mocking fetch or a local server)
  // - Config with missing optional fields (plugins, storage, etc.) handled by defaults
});