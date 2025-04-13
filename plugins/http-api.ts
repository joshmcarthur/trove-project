import { CoreSystem, EventFile, EventLink, Plugin } from "@trove/core/types.ts";

interface RouteHandler {
  method: string;
  path: string;
  handler: (request: Request, core: CoreSystem) => Promise<Response>;
}

const routes: RouteHandler[] = [
  {
    method: "POST",
    path: "/api/events",
    handler: async (request: Request, core: CoreSystem) => {
      try {
        // Check content type to determine how to parse the request
        const contentType = request.headers.get("Content-Type") || "";
        let body: Record<string, unknown>;

        if (contentType.includes("multipart/form-data")) {
          // Handle multipart form data
          const formData = await request.formData();
          body = {
            schema: formData.get("schema"),
            payload: JSON.parse(formData.get("payload") as string),
            producer: formData.get("producer"),
            metadata: formData.get("metadata")
              ? JSON.parse(formData.get("metadata") as string)
              : undefined,
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
        } else {
          // Handle JSON body
          body = await request.json();
        }

        // Validate required fields
        if (!body.schema) {
          return new Response("Schema is required", { status: 400 });
        }
        if (!body.payload) {
          return new Response("Payload is required", { status: 400 });
        }

        // Create the event
        const event = await core.createEvent(
          body.schema as string,
          body.payload as Record<string, unknown>,
          {
            producer: body.producer as string,
            files: body.files as EventFile[],
            links: body.links as EventLink[],
            metadata: body.metadata as Record<string, unknown>,
          },
        );

        // Return the created event
        return new Response(JSON.stringify(event), {
          status: 201,
          headers: {
            "Content-Type": "application/json",
          },
        });
      } catch (error) {
        core.logger.error("Error creating event:", error);
        return new Response("Error creating event", { status: 500 });
      }
    },
  },
];

export default {
  name: "http-api",
  version: "1.0.0",
  capabilities: [],
  hooks: {
    "http:request": async (context) => {
      const { request, core } = context;
      if (!request) {
        core.logger.warn("Received http:request hook with no request");
        return Promise.resolve();
      }

      const url = new URL(request.url);

      // Find matching route
      const route = routes.find(
        (r) => r.method === request.method && r.path === url.pathname,
      );

      if (route) {
        return route.handler(request, core);
      }

      return Promise.resolve();
    },
  },
} satisfies Plugin;
