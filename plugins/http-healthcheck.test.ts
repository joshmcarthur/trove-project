import { assertEquals } from "https://deno.land/std@0.220.1/assert/mod.ts";
import { createTestCore } from "@trove/core/test/utils.ts";
import { Plugin } from "@trove/core/types.ts";
import httpHealthcheckPlugin from "./http-healthcheck.ts";

Deno.test("http-healthcheck Plugin", async (t) => {
  await t.step("responds with OK for /up path", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpHealthcheckPlugin);

    // Create a mock request for /up path
    const request = new Request("http://localhost:3000/up");
    const context = { request, core };

    // Execute the hook directly
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result as Response;

    assertEquals(response.status, 200);
    assertEquals(await response.text(), "OK");
    assertEquals(response.headers.get("Content-Type"), "text/plain");
  });

  await t.step("resolves with void for non-/up paths", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpHealthcheckPlugin as unknown as Plugin);

    // Create a mock request for a different path
    const request = new Request("http://localhost:3000/other");
    const context = { request, core };

    // Execute the hook directly
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result;

    assertEquals(response, undefined);
  });
});
