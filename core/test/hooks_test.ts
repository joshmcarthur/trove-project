import { assertEquals } from "https://deno.land/std/testing/asserts.ts";
import { HookSystem } from "../hooks.ts";
import { TestLogger } from "./utils.ts";

Deno.test("HookSystem", async (t) => {
  await t.step("registers and executes hooks in priority order", async () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);
    const results: number[] = [];

    hooks.registerHook("test", "test:hook", () => {
      results.push(2);
      return Promise.resolve(2);
    }, 0);

    hooks.registerHook("test", "test:hook", () => {
      results.push(1);
      return Promise.resolve(1);
    }, 1);

    const hookResults = await hooks.executeHook("test:hook", {});
    assertEquals(results, [1, 2], "Hooks should execute in priority order");
    assertEquals(hookResults.length, 2, "Should return results from all hooks");
  });

  await t.step("handles errors in hooks gracefully", async () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);

    hooks.registerHook("test", "test:hook", () => {
      throw new Error("Test error");
    });

    const results = await hooks.executeHook("test:hook", {});
    assertEquals(results.length, 0, "Should not include failed hook results");
    assertEquals(
      logger.logs.some((log) =>
        log.level === "error" &&
        log.message.includes("Error executing hook test:hook")
      ),
      true,
      "Should log error",
    );
  });

  await t.step("unregisters plugins correctly", () => {
    const logger = new TestLogger();
    const hooks = new HookSystem(logger);

    hooks.registerHook("plugin1", "test:hook", () => Promise.resolve());
    hooks.registerHook("plugin2", "test:hook", () => Promise.resolve());
    hooks.unregisterPlugin("plugin1");

    // deno-lint-ignore no-explicit-any
    const hookSet = (hooks as any).hooks.get("test:hook");
    assertEquals(
      hookSet.size,
      1,
      "Should remove only the specified plugin's hooks",
    );
  });
});
