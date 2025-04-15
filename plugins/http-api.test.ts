import { assertEquals } from "https://deno.land/std@0.220.1/assert/mod.ts";
import { createTestCore } from "@trove/core/test/utils.ts";
import { Plugin } from "@trove/core/types.ts";
import httpApiPlugin from "./http-api.ts";

Deno.test("http-api Plugin", async (t) => {
  const schema = {
    $id: "test.event+v1.0.0",
    type: "object",
    properties: {
      message: { type: "string" },
    },
    required: ["message"],
  };

  await t.step("creates event for valid POST request with JSON", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpApiPlugin);

    // Create a mock request with valid event data
    const request = new Request("http://localhost:3000/api/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        schema,
        payload: { message: "Hello, world!" },
        producer: "test-producer",
      }),
    });

    const context = { request, core };
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result as Response;

    assertEquals(response.status, 201);
    assertEquals(response.headers.get("Content-Type"), "application/json");

    const event = await response.json();
    assertEquals(event.schema.$id, "test.event+v1.0.0");
    assertEquals(event.payload.message, "Hello, world!");
    assertEquals(event.producer, "test-producer");
  });

  await t.step(
    "creates event with files from multipart form data",
    async () => {
      const core = await createTestCore();
      await core.registerPlugin(httpApiPlugin as unknown as Plugin);

      // Create a mock file
      const file = new File(["test content"], "test.txt", {
        type: "text/plain",
      });

      // Create form data
      const formData = new FormData();
      formData.append("schema", JSON.stringify(schema));
      formData.append("payload", JSON.stringify({ message: "Hello, world!" }));
      formData.append("producer", "test-producer");
      formData.append("file1", file);

      // Create a mock request with multipart form data
      const request = new Request("http://localhost:3000/api/events", {
        method: "POST",
        body: formData,
      });

      const context = { request, core };
      const results = await core.executeHook("http:request", context);
      const response = results[0]?.result as Response;

      assertEquals(response.status, 201);
      assertEquals(response.headers.get("Content-Type"), "application/json");

      const event = await response.json();
      assertEquals(event.schema.$id, "test.event+v1.0.0");
      assertEquals(event.payload.message, "Hello, world!");
      assertEquals(event.producer, "test-producer");
      assertEquals(event.files.length, 1);
      assertEquals(event.files[0].filename, "test.txt");
      assertEquals(event.files[0].contentType, "text/plain");
    },
  );

  await t.step(
    "creates event with metadata from multipart form data",
    async () => {
      const core = await createTestCore();
      await core.registerPlugin(httpApiPlugin as unknown as Plugin);

      // Create form data with metadata
      const formData = new FormData();
      formData.append("schema", JSON.stringify(schema));
      formData.append("payload", JSON.stringify({ message: "Hello, world!" }));
      formData.append("metadata", JSON.stringify({ tags: ["test", "api"] }));

      // Create a mock request with multipart form data
      const request = new Request("http://localhost:3000/api/events", {
        method: "POST",
        body: formData,
      });

      const context = { request, core };
      const results = await core.executeHook("http:request", context);
      const response = results[0]?.result as Response;

      assertEquals(response.status, 201);
      assertEquals(response.headers.get("Content-Type"), "application/json");

      const event = await response.json();
      assertEquals(event.schema.$id, "test.event+v1.0.0");
      assertEquals(event.payload.message, "Hello, world!");
      assertEquals(event.metadata.tags, ["test", "api"]);
    },
  );

  await t.step(
    "returns 400 for invalid JSON payload in form data",
    async () => {
      const core = await createTestCore();
      await core.registerPlugin(httpApiPlugin as unknown as Plugin);

      // Create form data with invalid JSON payload
      const formData = new FormData();
      formData.append("schema", JSON.stringify(schema));
      formData.append("payload", "invalid json");
      formData.append("producer", "test-producer");

      const request = new Request("http://localhost:3000/api/events", {
        method: "POST",
        body: formData,
      });

      const context = { request, core };
      const results = await core.executeHook("http:request", context);
      const response = results[0]?.result as Response;

      assertEquals(response.status, 400);
      assertEquals(
        await response.text(),
        '{"error":"Invalid Request Format","details":"must have required property \'payload\'"}',
      );
    },
  );

  await t.step(
    "returns 400 for invalid JSON metadata in form data",
    async () => {
      const core = await createTestCore();
      await core.registerPlugin(httpApiPlugin as unknown as Plugin);

      // Create form data with invalid JSON metadata
      const formData = new FormData();
      formData.append("schema", JSON.stringify(schema));
      formData.append("payload", JSON.stringify({ message: "Hello, world!" }));
      formData.append("metadata", "invalid json");

      const request = new Request("http://localhost:3000/api/events", {
        method: "POST",
        body: formData,
      });

      const context = { request, core };
      const results = await core.executeHook("http:request", context);
      const response = results[0]?.result as Response;

      assertEquals(response.status, 400);
      assertEquals(
        await response.text(),
        '{"error":"Invalid Request Format","details":"Path /metadata: must be object"}',
      );
    },
  );

  await t.step("returns 400 for missing schema", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpApiPlugin as unknown as Plugin);

    const request = new Request("http://localhost:3000/api/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        payload: { message: "Hello, world!" },
      }),
    });

    const context = { request, core };
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result as Response;

    assertEquals(response.status, 400);
    assertEquals(
      await response.text(),
      '{"error":"Invalid Request Format","details":"must have required property \'schema\'"}',
    );
  });

  await t.step("returns 400 for missing payload", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpApiPlugin as unknown as Plugin);

    const request = new Request("http://localhost:3000/api/events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        schema,
      }),
    });

    const context = { request, core };
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result as Response;

    assertEquals(response.status, 400);
    assertEquals(
      await response.text(),
      '{"error":"Invalid Request Format","details":"must have required property \'payload\'"}',
    );
  });

  await t.step("ignores non-POST requests", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpApiPlugin as unknown as Plugin);

    const request = new Request("http://localhost:3000/api/events", {
      method: "GET",
    });

    const context = { request, core };
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result;

    assertEquals(response, undefined);
  });

  await t.step("ignores non-/api/events paths", async () => {
    const core = await createTestCore();
    await core.registerPlugin(httpApiPlugin as unknown as Plugin);

    const request = new Request("http://localhost:3000/other", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        schema,
        payload: { message: "Hello, world!" },
      }),
    });

    const context = { request, core };
    const results = await core.executeHook("http:request", context);
    const response = results[0]?.result;

    assertEquals(response, undefined);
  });
});
