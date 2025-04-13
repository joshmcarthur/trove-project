import { Hook, HookContext, HookHandler, HookResult, Logger } from "./types.ts";

export class HookSystem {
  private hooks: Map<string, Set<Hook>> = new Map();
  private logger: Logger;

  constructor(logger: Logger) {
    this.logger = logger;
  }

  registerHook(
    pluginId: string,
    hookName: string,
    handler: Hook | HookHandler,
    priority = 0,
  ): void {
    if (!this.hooks.has(hookName)) {
      this.hooks.set(hookName, new Set());
    }

    const hook: Hook = {
      pluginId,
      priority,
      handler: typeof handler === "function" ? handler : handler.handler,
    };

    this.hooks.get(hookName)!.add(hook);
    this.logger.debug(`Registered hook ${hookName} for plugin ${pluginId}`);
  }

  async executeHook(
    name: string,
    context: Partial<HookContext>,
  ): Promise<HookResult[]> {
    const hooks = this.hooks.get(name);
    if (!hooks || hooks.size === 0) {
      this.logger.debug(`No handlers registered for hook ${name}`);
      return [];
    }

    // Sort hooks by priority (higher numbers first)
    const sortedHooks = Array.from(hooks)
      .sort((a, b) => b.priority - a.priority);

    const results: HookResult[] = [];
    for (const hook of sortedHooks) {
      try {
        this.logger.debug(`Executing hook ${name} for plugin ${hook.pluginId}`);
        const result = await hook.handler(context as HookContext);
        results.push({ pluginId: hook.pluginId, result });
      } catch (error) {
        this.logger.error(
          `Error executing hook ${name} for plugin ${hook.pluginId}:`,
          error,
        );
      }
    }

    return results;
  }

  unregisterPlugin(pluginId: string): void {
    for (const [hookName, hooks] of this.hooks.entries()) {
      const filtered = new Set(
        Array.from(hooks).filter((hook) => hook.pluginId !== pluginId),
      );
      if (filtered.size === 0) {
        this.hooks.delete(hookName);
      } else {
        this.hooks.set(hookName, filtered);
      }
    }
  }
}
