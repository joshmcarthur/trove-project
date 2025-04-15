import { Logger } from "@trove/core/types.ts";

/**
 * Parses a JSON value from a FormDataEntryValue.
 * If the value is null, returns the missing value.
 * If the value is not parseable as JSON, returns the invalid value.
 */
export const safeParseJson = (
  value: FormDataEntryValue | null,
  { missing = undefined, invalid = undefined }: Record<string, unknown> = {},
  logger?: Logger,
): unknown | undefined => {
  if (!value) return missing;
  try {
    return JSON.parse(value as string);
  } catch (e) {
    logger?.debug(e as unknown as string);
    return invalid;
  }
};

export const parseRequestBody = async (
  request: Request,
  logger?: Logger,
): Promise<Record<string, unknown>> => {
  const contentType = request.headers.get("Content-Type") || "";

  if (contentType.includes("multipart/form-data")) {
    const formData = await request.formData();
    const body: Record<string, unknown> = {
      schema: safeParseJson(formData.get("schema"), {}, logger),
      payload: safeParseJson(formData.get("payload"), {}, logger),
      producer: (formData.get("producer") as string) || undefined,
      metadata: safeParseJson(
        formData.get("metadata"),
        { invalid: "" },
        logger,
      ),
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
    logger?.debug(e as unknown as string);
    return {};
  }
};
