import { JSONSchemaType } from "@trove/core/validator.ts";
import { EventFile, EventLink } from "@trove/core/types.ts";

// Define a local type that extends EventFile but overrides the data property,
// since a Uint8Array will be represented in a schema as an object.
type SchemaEventFile = Omit<EventFile, "data"> & {
  data: string | object;
};

// Define schemas for the complex types first
const eventFileSchema: JSONSchemaType<SchemaEventFile> = {
  type: "object",
  properties: {
    id: { type: "string" },
    contentType: { type: "string" },
    filename: { type: "string", nullable: true },
    size: { type: "number" },
    hash: { type: "string", nullable: true },
    data: { oneOf: [{ type: "string" }, { type: "object" }] },
    isReference: { type: "boolean", nullable: true },
  },
  required: ["contentType", "data"],
  additionalProperties: false,
};

const eventLinkSchema: JSONSchemaType<EventLink> = {
  type: "object",
  properties: {
    type: { type: "string" },
    targetEvent: {
      type: "object",
      properties: {
        id: { type: "string" },
        version: { type: "number", nullable: true },
      },
      required: ["id"],
    },
    metadata: { type: "object", nullable: true, additionalProperties: true },
  },
  required: ["type", "targetEvent"],
  additionalProperties: false,
};

// Define the type that matches our schema
export interface CreateEventRequest {
  schema: Record<string, unknown>;
  payload: Record<string, unknown>;
  producer?: string;
  metadata?: Record<string, unknown>;
  files?: SchemaEventFile[];
  links?: EventLink[];
}

// Create a type-safe schema
export const requestSchema: JSONSchemaType<CreateEventRequest> = {
  type: "object",
  properties: {
    schema: {
      type: "object",
      additionalProperties: true,
    },
    payload: { type: "object", additionalProperties: true },
    producer: { type: "string", nullable: true },
    metadata: { type: "object", nullable: true, additionalProperties: true },
    files: {
      type: "array",
      nullable: true,
      items: eventFileSchema,
    },
    links: {
      type: "array",
      nullable: true,
      items: eventLinkSchema,
    },
  },
  required: ["schema", "payload"],
  additionalProperties: false,
};
