import { CoreSystem, EventFile } from "@trove/core/types.ts";
import { parseRequestBody } from "./body-parser.ts";
import { CreateEventRequest, requestSchema } from "./schema.ts";

export const createEventHandler = async (
  request: Request,
  core: CoreSystem,
): Promise<Response> => {
  try {
    const body = await parseRequestBody(request, core.logger);
    const validationResult = core.validator.validate<CreateEventRequest>(
      requestSchema,
      body,
    );

    if (!validationResult.isValid || !validationResult.data) {
      return new Response(
        JSON.stringify({
          error: "Invalid Request Format",
          details: core.validator.formatErrors(validationResult.errors || []),
        }),
        {
          status: 400,
          headers: { "Content-Type": "application/json" },
        },
      );
    }

    const data = validationResult.data;

    try {
      const event = await core.createEvent(data.schema, data.payload, {
        producer: data.producer,
        files: data.files as EventFile[],
        links: data.links,
        metadata: data.metadata,
      });

      return new Response(JSON.stringify(event), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      });
    } catch (error: unknown) {
      if (
        error instanceof Error && error.message.includes("validation failed")
      ) {
        core.logger.error(error as unknown as string);
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
    return new Response(JSON.stringify({ error: "Internal Server Error" }), {
      status: 500,
    });
  }
};
