import { Plugin } from "@trove/core/types.ts";

export default {
  name: "http-healthcheck",
  version: "1.0.0",
  capabilities: [],
  hooks: {
    "http:request": (context) => {
      const request = context.request;
      if (!request) {
        context.core.logger.warn("Received http:request hook with no request");
        return Promise.resolve();
      }

      const url = new URL(request.url);

      if (url.pathname === "/up") {
        return Promise.resolve(
          new Response("OK", {
            status: 200,
            headers: {
              "Content-Type": "text/plain",
            },
          }),
        );
      }

      return Promise.resolve();
    },
  },
} satisfies Plugin;
