import { assertEquals } from "https://deno.land/std@0.220.1/assert/mod.ts";
import {
  assertSpyCalls,
  stub,
} from "https://deno.land/std@0.220.1/testing/mock.ts";
import { run } from "../cli.ts";
import { TestLogger } from "./utils.ts";

Deno.test("run", async (t) => {
  const logger = new TestLogger();

  await t.step("initializes and shuts down Trove on SIGINT", async () => {
    const exitStub = stub(Deno, "exit");
    const addSignalListenerStub = stub(
      Deno,
      "addSignalListener",
      (_signal: Deno.Signal, handler: () => void) => handler(),
    );

    try {
      const configPath = "./core/test/fixtures/cli.config.ts";
      await run(["--config", configPath], logger);

      assertSpyCalls(exitStub, 1);
      assertEquals(exitStub.calls[0].args[0], 0);

      assertSpyCalls(addSignalListenerStub, 1);
      assertEquals(addSignalListenerStub.calls[0].args[0], "SIGINT");
    } finally {
      exitStub.restore();
      addSignalListenerStub.restore();
    }
  });

  await t.step("exits with error on invalid config", async () => {
    const exitStub = stub(Deno, "exit");

    try {
      const configPath = "nonexistent.config.ts";
      await run(["--config", configPath], logger);

      // Assert that exit was called with error code
      assertSpyCalls(exitStub, 1);
      assertEquals(exitStub.calls[0].args[0], 1);
    } finally {
      exitStub.restore();
    }
  });
});
