import { Plugin } from "@trove/core/types.ts";
import { route } from "./http-api/routes.ts";

export default {
  name: "http-api",
  version: "1.0.0",
  capabilities: [],
  hooks: {
    "http:request": (context) => {
      const { request, core } = context;
      if (!request) return Promise.resolve();

      return route(request)?.handler(request, core) ?? Promise.resolve();
    },
  },
} satisfies Plugin;
