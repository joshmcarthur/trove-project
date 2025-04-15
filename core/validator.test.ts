import {
  assert,
  assertEquals,
} from "https://deno.land/std@0.220.1/assert/mod.ts";
import { Validator } from "./validator.ts";

Deno.test("Validator", async (t) => {
  await t.step("validates basic schema", () => {
    const validator = new Validator();
    const schema = {
      type: "object",
      properties: {
        name: { type: "string" },
        age: { type: "number" },
      },
      required: ["name", "age"],
    };

    // Valid data
    const validData = { name: "John", age: 30 };
    const validResult = validator.validate(schema, validData);
    assert(validResult.isValid);
    assertEquals(validResult.data, validData);

    // Invalid data
    const invalidData = { name: "John" };
    const invalidResult = validator.validate(schema, invalidData);
    assert(!invalidResult.isValid);
    assert(invalidResult.errors);
    assertEquals(invalidResult.errors.length, 1);
  });

  await t.step("formats validation errors", () => {
    const validator = new Validator();
    const schema = {
      type: "object",
      properties: {
        name: { type: "string" },
        age: { type: "number" },
      },
      required: ["name", "age"],
    };

    const invalidData = { name: "John" };
    const result = validator.validate(schema, invalidData);

    assert(!result.isValid);
    assert(result.errors);
    assertEquals(result.errors.length, 1);
    assertEquals(result.errors[0].path, "");
    assertEquals(result.errors[0].message, "must have required property 'age'");
    assertEquals(result.errors[0].extra?.keyword, "required");

    const formattedErrors = validator.formatErrors(result.errors);
    assert(formattedErrors.includes("must have required property 'age'"));
  });

  await t.step("validates nested objects", () => {
    const validator = new Validator();
    const schema = {
      type: "object",
      properties: {
        user: {
          type: "object",
          properties: {
            name: { type: "string" },
            address: {
              type: "object",
              properties: {
                street: { type: "string" },
                city: { type: "string" },
              },
              required: ["street", "city"],
            },
          },
          required: ["name", "address"],
        },
      },
      required: ["user"],
    };

    // Valid data
    const validData = {
      user: {
        name: "John",
        address: {
          street: "123 Main St",
          city: "Anytown",
        },
      },
    };
    const validResult = validator.validate(schema, validData);
    assert(validResult.isValid);
    assertEquals(validResult.data, validData);

    // Invalid data
    const invalidData = {
      user: {
        name: "John",
        address: {
          street: "123 Main St",
          // missing city
        },
      },
    };
    const invalidResult = validator.validate(schema, invalidData);
    assert(!invalidResult.isValid);
    assert(invalidResult.errors);
    assertEquals(invalidResult.errors.length, 1);
    assertEquals(invalidResult.errors[0].path, "/user/address");
    assertEquals(
      invalidResult.errors[0].message,
      "must have required property 'city'",
    );
  });

  await t.step("validates arrays", () => {
    const validator = new Validator();
    const schema = {
      type: "array",
      items: {
        type: "object",
        properties: {
          id: { type: "string" },
          value: { type: "number" },
        },
        required: ["id", "value"],
      },
    };

    // Valid data
    const validData = [
      { id: "1", value: 10 },
      { id: "2", value: 20 },
    ];
    const validResult = validator.validate(schema, validData);
    assert(validResult.isValid);
    assertEquals(validResult.data, validData);

    // Invalid data
    const invalidData = [
      { id: "1", value: 10 },
      { id: "2" }, // missing value
    ];
    const invalidResult = validator.validate(schema, invalidData);
    assert(!invalidResult.isValid);
    assert(invalidResult.errors);
    assertEquals(invalidResult.errors.length, 1);
    assertEquals(invalidResult.errors[0].path, "/1");
    assertEquals(
      invalidResult.errors[0].message,
      "must have required property 'value'",
    );
  });

  await t.step("handles multiple validation errors", () => {
    const validator = new Validator();
    const schema = {
      type: "object",
      properties: {
        name: { type: "string" },
        age: { type: "number" },
        email: { type: "string" },
      },
      required: ["name", "age", "email"],
    };

    const invalidData = {
      name: 123, // wrong type
      age: "30", // wrong type
      // missing email
    };

    const result = validator.validate(schema, invalidData);
    assert(!result.isValid);
    assert(result.errors);
    assert(
      result.errors.length >= 3,
      `Expected 3 errors, got ${result.errors.length}`,
    );

    // Check for specific errors
    const errorMessages = result.errors.map((e) => e.message);
    assert(errorMessages.some((m) => m.includes("must be string"))); // name
    assert(errorMessages.some((m) => m.includes("must be number"))); // age
    assert(errorMessages.some((m) => m.includes("required property"))); // email
  });
});
