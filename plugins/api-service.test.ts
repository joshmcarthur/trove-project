import { assertEquals } from "https://deno.land/std@0.220.1/assert/mod.ts";
import { createTestCore } from "@trove/core/test/utils.ts";
import { Plugin } from "@trove/core/types.ts";
import apiPlugin from "./api.ts";

Deno.test("API Service Plugin", async (t) => {
  await t.step("initializes and starts server", async () => {
    const core = await createTestCore({
      plugins: {
        sources: [],
        config: {
          "api": {
            port: 3001, // Use different port for tests
          },
        },
      },
    });

    await core.registerPlugin(apiPlugin as unknown as Plugin);
    const plugin = await core.getPlugin("api");
    assertEquals(plugin?.name, "api");

    // Wait for server to start
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Test creating an event via API
    const response = await fetch("http://localhost:3001/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        schema: "test.event",
        payload: {
          message: "Hello from test",
        },
      }),
    });

    const event = await response.json();
    assertEquals(response.status, 201);
    assertEquals(event.schema.id, "test.event");
    assertEquals(event.payload.message, "Hello from test");
    assertEquals(event.producer, "api");

    // Test 404 for unknown route
    const notFoundResponse = await fetch("http://localhost:3001/unknown", {
      method: "GET",
    });
    assertEquals(notFoundResponse.status, 404);

    // Test invalid request
    const invalidResponse = await fetch("http://localhost:3001/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        // Missing required schema
        payload: {
          message: "Invalid request",
        },
      }),
    });
    assertEquals(invalidResponse.status, 400);

    // Cleanup
    notFoundResponse.body?.cancel();
    invalidResponse.body?.cancel();
    await core.shutdown();
  });

  await t.step("handles request hooks", async () => {
    const core = await createTestCore({
      plugins: {
        sources: [],
        config: {
          "api": {
            port: 3002, // Different port for this test
          },
        },
      },
    });

    // Register API plugin first
    await core.registerPlugin(apiPlugin as unknown as Plugin);

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
            url.pathname === "/events" &&
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
    const interceptedResponse = await fetch("http://localhost:3002/events", {
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

  await t.step("handles response hooks", async () => {
    const core = await createTestCore({
      plugins: {
        sources: [],
        config: {
          "api": {
            port: 3003, // Different port for this test
          },
        },
      },
    });

    // Register API plugin first
    await core.registerPlugin(apiPlugin as unknown as Plugin);

    // Register a plugin that modifies responses
    await core.registerPlugin({
      name: "test-response-modifier",
      version: "1.0.0",
      capabilities: [],
      hooks: {
        "http:response": async (context) => {
          const response = context.response;
          if (!response) return null;

          if (response.status === 201) {
            try {
              const data = await response.clone().json();
              return new Response(
                JSON.stringify({
                  ...data,
                  modified: true,
                }),
                {
                  status: 201,
                  headers: {
                    "Content-Type": "application/json",
                    "X-Modified": "true",
                  },
                },
              );
            } catch (error) {
              console.error("Error parsing response:", error);
              return null;
            }
          }
          console.log("Not modifying response");
          return null;
        },
      },
    });

    // Wait for server to start
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Test that hook modifies the response
    const response = await fetch("http://localhost:3003/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        schema: "test.event",
        payload: { message: "Test response hook" },
      }),
    });

    const data = await response.json();
    assertEquals(response.status, 201);
    assertEquals(response.headers.get("X-Modified"), "true");
    assertEquals(data.modified, true);
    assertEquals(data.payload.message, "Test response hook");

    // Cleanup
    await core.shutdown();
  });
});
