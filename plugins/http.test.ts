import { assertEquals } from "https://deno.land/std@0.220.1/assert/mod.ts";
import { createTestCore } from "@trove/core/test/utils.ts";
import { Plugin } from "@trove/core/types.ts";
import httpPlugin from "./http.ts";

Deno.test("http Service Plugin", async (t) => {
  await t.step("initializes and starts server", async () => {
    const core = await createTestCore({
      plugins: {
        sources: [],
        config: {
          "http": {
            port: 3001, // Use different port for tests
          },
        },
      },
    });

    await core.registerPlugin(httpPlugin as unknown as Plugin);
    const plugin = await core.getPlugin("http");
    assertEquals(plugin?.name, "http");

    // Wait for server to start
    await new Promise((resolve) => setTimeout(resolve, 100));

    // // Test healthcheck endpoint
    // const response = await fetch("http://localhost:3001/");

    // assertEquals(response.status, 200);
    // assertEquals(await response.text(), "OK");

    // Test 404 for unknown route
    const notFoundResponse = await fetch("http://localhost:3001/unknown");
    assertEquals(notFoundResponse.status, 404);
    assertEquals(await notFoundResponse.text(), "Not Found");
    await core.shutdown();
  });

  await t.step("handles http:request hooks", async () => {
    const core = await createTestCore({
      plugins: {
        sources: [],
        config: {
          "http": {
            port: 3002, // Different port for this test
          },
        },
      },
    });

    // Register http plugin first
    await core.registerPlugin(httpPlugin as unknown as Plugin);

    // Register a plugin that intercepts requests
    await core.registerPlugin({
      name: "test-interceptor",
      version: "1.0.0",
      capabilities: [],
      hooks: {
        "http:request": async (context) => {
          const request = context.request;
          if (!request) return null;

          const url = new URL(request.url);

          if (
            url.pathname === "/up" &&
            request.headers.get("X-Test-Intercept") === "true"
          ) {
            console.log("Intercepting request");
            return new Response("Intercepted", {
              status: 200,
              headers: {
                "Content-Type": "text/plain",
              },
            });
          }
          console.log("Not intercepting request");
          return null;
        },
      },
    });

    // Wait for server to start
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Test that hook intercepts the request
    const interceptedResponse = await fetch("http://localhost:3002/up", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Test-Intercept": "true",
      },
      body: JSON.stringify({
        schema: "test.event",
        payload: { message: "Should not reach handler" },
      }),
    });

    assertEquals(interceptedResponse.status, 200);
    assertEquals(await interceptedResponse.text(), "Intercepted");

    // Cleanup
    await core.shutdown();
  });
});
