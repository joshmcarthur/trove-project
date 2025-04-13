import { CoreSystem, Plugin } from "@trove/core/types.ts";
import { serve } from "https://deno.land/std@0.220.1/http/server.ts";

interface HttpPlugin extends Plugin {
  controller: AbortController | null;
  listening(core: CoreSystem, { port }: { port: number }): void;
  handleRequest(req: Request, core: CoreSystem): Promise<Response>;
}

interface PluginConfig {
  port?: number;
}

export default {
  name: "http",
  version: "1.0.0",
  capabilities: [],
  // Server instance
  controller: null as AbortController | null,

  initialize(core: CoreSystem) {
    // Get port from config, default to 3000
    const config = (core.config.plugins.config?.["http"] || {}) as PluginConfig;
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
          core.logger.error("HTTP server error:", error);
        }
      }
    })();

    return Promise.resolve();
  },

  listening(core: CoreSystem, { port }: { port: number }) {
    core.logger.info(`HTTP service started on port ${port}`);
    core.executeHook("http:listening", { core, state: { port } });
  },

  shutdown() {
    if (this.controller) {
      this.controller.abort();
      this.controller = null;
    }

    return Promise.resolve();
  },

  async handleRequest(req: Request, core: CoreSystem): Promise<Response> {
    // Execute request hook
    const results = await core.executeHook("http:request", {
      request: req,
      core,
    });

    // Find first non-null response from hooks
    const response = results.find((result) => result.result !== null)
      ?.result as Response | undefined;
    if (response) {
      return response;
    }

    // Return 404 for unhandled requests
    return new Response("Not Found", { status: 404 });
  },
} satisfies HttpPlugin;
