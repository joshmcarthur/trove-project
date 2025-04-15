import { CoreSystem } from "@trove/core/types.ts";
import { createEventHandler } from "./handlers/create-event/index.ts";

export type RouteHandler = (
  request: Request,
  core: CoreSystem,
) => Promise<Response>;

export interface Route {
  method: string;
  path: string;
  handler: RouteHandler;
}

export const routes: Route[] = [
  {
    method: "POST",
    path: "/api/events",
    handler: createEventHandler,
  },
];

export const route = (request: Request): Route | undefined => {
  const url = new URL(request.url);
  return routes.find((r) =>
    r.method === request.method && r.path === url.pathname
  );
};
