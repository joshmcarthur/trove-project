import Ajv from "https://esm.sh/ajv@8.12.0";
import { ValidationError, ValidationResult } from "./types.ts";

export class Validator {
  private ajv: Ajv;

  constructor() {
    this.ajv = new Ajv({
      allErrors: true,
      strict: true,
      coerceTypes: false,
      removeAdditional: true,
      allowUnionTypes: true,
    });
  }

  validate<T = unknown>(
    schema: Record<string, unknown>,
    data: unknown,
  ): ValidationResult<T> {
    // First validate that the schema itself is valid
    try {
      this.ajv.compile(schema);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "Unknown error";
      return {
        isValid: false,
        errors: [{
          path: "",
          message: `Invalid schema: ${message}`,
          extra: { keyword: "schema" },
        }],
      };
    }

    const validate = this.ajv.compile(schema);
    const isValid = validate(data) as boolean;

    if (!isValid) {
      return {
        isValid: false,
        errors: (validate.errors || []).map((error) => ({
          path: error.instancePath,
          message: error.message ?? "Validation error",
          extra: {
            keyword: error.keyword,
            params: error.params,
          },
        })),
      };
    }

    return {
      isValid: true,
      data: data as T,
    };
  }

  formatErrors(errors: ValidationError[]): string {
    return errors.map((error) =>
      `${error.path ? `Path ${error.path}: ` : ""}${error.message}`
    ).join("\n");
  }
}
