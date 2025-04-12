import { CoreSystem, Plugin } from "@trove/core/types.ts";
import { serve } from "https://deno.land/std@0.220.1/http/server.ts";

type PluginCapability =
  | "http:handler"
  | "storage:events"
  | "storage:files"
  | "storage:links";

interface ApiServicePlugin extends Omit<Plugin, "capabilities"> {
  capabilities: PluginCapability[];
  controller: AbortController | null;
  listening(core: CoreSystem, { port }: { port: number }): void;
  handleRequest(req: Request, core: CoreSystem): Promise<Response>;
}

interface PluginConfig {
  port?: number;
}

interface CoreConfig {
  plugins: {
    sources: string[];
    config?: Record<string, PluginConfig>;
  };
}

export default {
  name: "api",
  version: "1.0.0",
  capabilities: ["http:handler"] as PluginCapability[],

  // Server instance
  controller: null as AbortController | null,

  async initialize(core: CoreSystem) {
    // Get port from config, default to 3000
    const config = (core.config as CoreConfig).plugins.config?.["api"] || {};
    const port = config.port || 3000;

    // Start HTTP server
    const controller = new AbortController();
    this.controller = controller;

    // Start server in background
    (async () => {
      try {
        await serve(
          (req) => this.handleRequest(req, core),
          {
            port,
            onListen: () => this.listening(core, { port }),
            signal: controller.signal,
          },
        );
      } catch (error: unknown) {
        if (error instanceof Error && error.name !== "AbortError") {
          core.logger.error("API server error:", error);
        }
      }
    })();
  },

  listening(core: CoreSystem, { port }: { port: number }) {
    core.logger.info(`API service started on port ${port}`);
    core.executeHook("api:listening", { core, state: { port } });
  },

  shutdown() {
    if (this.controller) {
      this.controller.abort();
      this.controller = null;
    }

    return Promise.resolve();
  },

  async handleRequest(req: Request, core: CoreSystem): Promise<Response> {
    const url = new URL(req.url);

    // Execute pre-request hook
    const preResults = await core.executeHook("http:request", {
      request: req,
      core,
    });

    // Find first non-null response from hooks
    const preResponse = preResults.find((result) => result.result !== null)
      ?.result;
    if (preResponse) {
      return preResponse as Response;
    }

    // Handle POST /events
    if (req.method === "POST" && url.pathname === "/events") {
      try {
        const eventData = await req.json();

        // Create event using core
        const event = await core.createEvent(
          eventData.schema,
          eventData.payload,
          {
            files: eventData.files || [],
            links: eventData.links || [],
            metadata: eventData.metadata,
            producer: "api",
          },
        );

        // Create initial response
        const response = new Response(JSON.stringify(event), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        });

        // Execute response hook
        const postResults = await core.executeHook("http:response", {
          request: req,
          response,
          core,
        });

        // Find first non-null response from hooks
        const modifiedResponse = postResults.find((result) =>
          result.result !== null
        )?.result;
        return modifiedResponse ? (modifiedResponse as Response) : response;
      } catch (error: unknown) {
        const errorMessage = error instanceof Error
          ? error.message
          : "Unknown error";
        return new Response(JSON.stringify({ error: errorMessage }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }
    }

    // Return 404 for unknown routes
    return new Response("Not Found", { status: 404 });
  },
} satisfies ApiServicePlugin;
