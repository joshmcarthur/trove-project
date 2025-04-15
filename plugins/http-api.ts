import {
  CoreSystem,
  EventFile,
  EventLink,
  Logger,
  Plugin,
} from "@trove/core/types.ts";

class HttpApiPlugin {
  logger?: Logger;

  private static readonly requestSchema = {
    type: "object",
    properties: {
      schema: {
        type: "object",
        additionalProperties: true,
      },
      payload: { type: "object", additionalProperties: true },
      producer: { type: "string" },
      metadata: { type: "object", additionalProperties: true },
      files: {
        type: "array",
        items: {
          type: "object",
          properties: {
            contentType: { type: "string" },
            filename: { type: "string" },
            size: { type: "number" },
            // Uint8Array or base64 encoded string - note that Uint8Array is a object type,
            // not an array
            data: { type: ["string", "object"] },
          },
          required: ["contentType", "data"],
        },
      },
      links: {
        type: "array",
        items: {
          type: "object",
          properties: {
            type: { type: "string" },
            targetEvent: {
              type: "object",
              properties: {
                id: { type: "string" },
                version: { type: "number" },
              },
              required: ["id"],
            },
          },
          required: ["type", "targetEvent"],
        },
      },
    },
    required: ["schema", "payload"],
    additionalProperties: false,
  };

  /**
   * Parses a JSON value from a FormDataEntryValue.
   * If the value is null, returns the missing value.
   * If the value is not parseable as JSON, returns the invalid value.
   * @param value - The value to parse.
   * @param options - An object containing the missing and invalid values, defaults to undefined.
   * @returns The parsed value, or the missing or invalid value if the value is null or not a string.
   */
  private safeParseJson(
    value: FormDataEntryValue | null,
    { missing = undefined, invalid = undefined }: Record<string, unknown> = {},
  ): unknown | undefined {
    if (!value) return missing;
    try {
      return JSON.parse(value as string);
    } catch (e) {
      this.logger?.debug(e as unknown as string);
      return invalid;
    }
  }

  private async parseRequestBody(
    request: Request,
  ): Promise<Record<string, unknown>> {
    const contentType = request.headers.get("Content-Type") || "";

    if (contentType.includes("multipart/form-data")) {
      const formData = await request.formData();
      const body: Record<string, unknown> = {
        schema: this.safeParseJson(formData.get("schema")),
        payload: this.safeParseJson(formData.get("payload")),
        producer: (formData.get("producer") as string) || undefined,
        metadata: this.safeParseJson(formData.get("metadata"), { invalid: "" }),
      };

      // Handle file uploads if present
      const files = [];
      for (const [key, value] of formData.entries()) {
        if (value instanceof File) {
          files.push({
            id: key,
            contentType: value.type,
            filename: value.name,
            size: value.size,
            data: new Uint8Array(await value.arrayBuffer()),
          });
        }
      }
      if (files.length > 0) {
        body.files = files;
      }

      return body;
    }

    // For JSON requests, wrap in try/catch to handle invalid JSON
    try {
      return await request.json();
    } catch (e) {
      this.logger?.debug(e as unknown as string);
      return {};
    }
  }

  private routes = [
    {
      method: "POST",
      path: "/api/events",
      handler: async (request: Request, core: CoreSystem) => {
        this.logger = core.logger;

        try {
          // Parse request body
          const body = (await this.parseRequestBody(request)) as {
            schema: Record<string, unknown>;
            payload: Record<string, unknown>;
            producer?: string;
            files?: EventFile[];
            links?: EventLink[];
            metadata?: Record<string, unknown>;
          };

          // Validate request format
          const validationResult = core.validator.validate(
            HttpApiPlugin.requestSchema,
            body,
          );

          if (!validationResult.isValid) {
            return new Response(
              JSON.stringify({
                error: "Invalid Request Format",
                details: core.validator.formatErrors(
                  validationResult.errors || [],
                ),
              }),
              {
                status: 400,
                headers: { "Content-Type": "application/json" },
              },
            );
          }

          // Create the event
          try {
            const event = await core.createEvent(body.schema, body.payload, {
              producer: body.producer,
              files: body.files,
              links: body.links,
              metadata: body.metadata,
            });

            return new Response(JSON.stringify(event), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            });
          } catch (error: unknown) {
            // Handle validation errors from event creation
            if (
              error instanceof Error &&
              error.message.includes("validation failed")
            ) {
              this.logger?.error(error as unknown as string);
              return new Response(
                JSON.stringify({
                  error: "Invalid Event Payload",
                  details: error.message,
                }),
                {
                  status: 422,
                  headers: { "Content-Type": "application/json" },
                },
              );
            }
            throw error;
          }
        } catch (error: unknown) {
          console.error(error);
          core.logger.error("Error handling request:", error);
          return new Response(
            JSON.stringify({ error: "Internal Server Error" }),
            { status: 500 },
          );
        }
      },
    },
  ];

  public plugin: Plugin = {
    name: "http-api",
    version: "1.0.0",
    capabilities: [],
    hooks: {
      "http:request": (context) => {
        const { request, core } = context;
        if (!request) return Promise.resolve();

        const url = new URL(request.url);
        const route = this.routes.find((r) =>
          r.method === request.method && r.path === url.pathname
        );

        return route ? route.handler(request, core) : Promise.resolve();
      },
    },
  };
}

const httpApiPlugin = new HttpApiPlugin();
export default httpApiPlugin.plugin;
