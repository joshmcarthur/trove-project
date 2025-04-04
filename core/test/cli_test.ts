import {
  assertEquals,
  assertRejects,
} from "https://deno.land/std@0.220.1/assert/mod.ts";
import { loadConfig, run } from "../cli.ts";

Deno.test("loadConfig", async (t) => {
  await t.step("loads valid configuration", async () => {
    const configPath = "./test/fixtures/cli.config.ts";
    const config = await loadConfig(configPath);
    assertEquals(config.plugins.directories, ["plugins"]);
  });

  await t.step("throws error on invalid configuration", async () => {
    const configPath = "./core/test/fixtures/nonexistent.config.ts";
    await assertRejects(
      () => loadConfig(configPath),
      Error,
      "Failed to load configuration",
    );
  });
});

Deno.test("run", async (t) => {
  await t.step("initializes and shuts down Trove on SIGINT", async () => {
    // Store original functions
    const originalAddSignalListener = Deno.addSignalListener;
    const originalExit = Deno.exit;

    try {
      // Mock addSignalListener to immediately trigger the callback
      Deno.addSignalListener = (_signal: Deno.Signal, handler: () => void) => {
        // Schedule the handler to run after current execution
        queueMicrotask(handler);
      };

      // Mock exit
      let exitCode: number | undefined;
      let exitCalled = false;
      // @ts-ignore: Deno.exit is being mocked
      Deno.exit = (code?: number) => {
        exitCode = code;
        exitCalled = true;
      };

      const configPath = "./core/test/fixtures/cli.config.ts";
      await run(["--config", configPath]);

      assertEquals(exitCode, 0);
      assertEquals(exitCalled, true);
    } finally {
      // Restore original functions
      Deno.addSignalListener = originalAddSignalListener;
      Deno.exit = originalExit;
    }
  });

  await t.step("exits with error on invalid config", async () => {
    const originalExit = Deno.exit;
    let exitCode: number | undefined;
    let exitCalled = false;

    try {
      // @ts-ignore: Deno.exit is being mocked
      Deno.exit = (code?: number) => {
        exitCode = code;
        exitCalled = true;
      };

      const configPath = "nonexistent.config.ts";
      await run(["--config", configPath]);

      assertEquals(exitCode, 1);
      assertEquals(exitCalled, true);
    } finally {
      Deno.exit = originalExit;
    }
  });
});
