import {
  assertEquals,
  assertRejects,
} from "https://deno.land/std/assert/mod.ts";
import { createTestCore } from "./utils.ts";

Deno.test("Trove", async (t) => {
  await t.step("initializes and shuts down correctly", async () => {
    const trove = await createTestCore();

    // any-casting to access private property 'initialized'
    // deno-lint-ignore no-explicit-any
    assertEquals((trove as any).initialized, true);

    await trove.shutdown();

    // any-casting to access private property 'initialized'
    // deno-lint-ignore no-explicit-any
    assertEquals((trove as any).initialized, false);
  });

  await t.step("prevents double initialization", async () => {
    const trove = await createTestCore();

    await assertRejects(
      () => trove.initialize(),
      Error,
      "Trove is already initialized",
    );
  });

  await t.step("creates and retrieves events", async () => {
    const trove = await createTestCore();

    const testEvent = await trove.createEvent(
      {
        type: "object",
        properties: {
          test: { type: "string" },
        },
        required: ["test"],
      },
      { test: "data" },
    );

    const retrievedEvent = await trove.getEvent(testEvent.id);
    assertEquals(retrievedEvent?.payload, { test: "data" });
  });

  await t.step("executes hooks during event lifecycle", async () => {
    const trove = await createTestCore();

    let storingCalled = false;
    let storedCalled = false;

    const testPlugin = {
      name: "test-plugin",
      version: "1.0.0",
      capabilities: [],
      hooks: {
        "event:storing": () => {
          storingCalled = true;
          return Promise.resolve();
        },
        "event:stored": () => {
          storedCalled = true;
          return Promise.resolve();
        },
      },
    };

    await trove.registerPlugin(testPlugin);

    await trove.createEvent(
      {
        type: "object",
        properties: {
          test: { type: "string" },
        },
        required: ["test"],
      },
      { test: "data" },
    );

    assertEquals(storingCalled, true, "event:storing hook should be called");
    assertEquals(storedCalled, true, "event:stored hook should be called");
  });

  await t.step("validates event schema and payload", async () => {
    const trove = await createTestCore();
    const schema = {
      type: "object",
      properties: {
        name: { type: "string" },
        value: { type: "number" },
      },
    };

    // Test with valid schema and payload
    const validEvent = await trove.createEvent(schema, {
      name: "Test Event",
      value: 42,
    });

    assertEquals(validEvent.schema, schema);
    assertEquals(validEvent.payload.name, "Test Event");
    assertEquals(validEvent.payload.value, 42);

    // Test with invalid payload (missing required field)
    await assertRejects(
      async () => {
        await trove.createEvent(
          {
            type: "object",
            properties: {
              name: { type: "string" },
              value: { type: "number" },
            },
            required: ["name", "value"],
          },
          { name: "Test Event" }, // missing required 'value' field
        );
      },
      Error,
      "validation failed",
    );

    // Test with invalid schema (wrong type)
    await assertRejects(
      async () => {
        await trove.createEvent(
          {
            type: "object",
            properties: {
              name: { type: "string" },
              value: { type: "number" },
            },
            required: ["name", "value"],
          },
          { name: "Test Event", value: "not a number" },
        );
      },
      Error,
      "validation failed",
    );
  });
});
