import { Validator } from "./validator.ts";

export interface EventId {
  id: string;
  version?: number;
}

export interface EventSchema {
  id: string;
  version: string;
}

export interface EventFile {
  id: string;
  contentType: string;
  filename?: string;
  size: number;
  hash?: string;
  data: Uint8Array | string;
  isReference?: boolean;
}

export interface EventLink {
  type: string;
  targetEvent: EventId;
  metadata?: Record<string, unknown>;
}

export interface Event {
  id: EventId;
  createdAt: string;
  producer: string;
  schema: Record<string, unknown>;
  payload: Record<string, unknown>;
  files: EventFile[];
  links: EventLink[];
  metadata?: Record<string, unknown>;
}

export interface HookContext {
  core: CoreSystem;
  event?: Event;
  request?: Request;
  state?: Record<string, unknown>;
}

export interface HookResult<T = unknown> {
  pluginId: string;
  result: T | null;
}

export interface Hook {
  pluginId: string;
  priority: number;
  handler: (context: HookContext) => Promise<unknown>;
}

export type PluginCapability =
  | "storage:events"
  | "storage:files"
  | "storage:links";

export interface Plugin {
  name: string;
  version: string;
  capabilities: PluginCapability[];
  hooks?: Record<string, Hook | HookHandler>;
  initialize?: (core: CoreSystem) => Promise<void>;
  shutdown?: () => Promise<void>;
}

export type HookHandler = (context: HookContext) => Promise<unknown>;

export type ValidationError = {
  path: string;
  message: string;
  extra?: Record<string, unknown>;
};

export interface ValidationResult<T = unknown> {
  isValid: boolean;
  data?: T;
  errors?: ValidationError[];
}

export interface Logger {
  debug(message: string, ...args: unknown[]): void;
  info(message: string, ...args: unknown[]): void;
  warn(message: string, ...args: unknown[]): void;
  error(message: string, ...args: unknown[]): void;
}

export interface CoreConfig {
  plugins: {
    sources: string[];
    config?: Record<string, unknown>;
  };
  storage: StorageConfiguration;
  logger?: Logger;
}

export interface BaseStoragePlugin {
  initialize(options: unknown): Promise<void>;
}

export interface EventStorage extends BaseStoragePlugin {
  saveEvent(event: Event): Promise<Event>;
  getEvent(id: EventId): Promise<Event | null>;
  queryEvents(query: EventQuery): Promise<Event[]>;
}

export interface FileStorage extends BaseStoragePlugin {
  saveFile(file: EventFile): Promise<string>;
  getFile(fileId: string): Promise<EventFile | null>;
  getFileData(fileId: string): Promise<Uint8Array | string>;
}

export interface LinkStorage extends BaseStoragePlugin {
  saveLink(sourceEventId: EventId, link: EventLink): Promise<void>;
  getLinks(eventId: EventId, options?: { type?: string }): Promise<EventLink[]>;
}

export interface StoragePluginConfiguration {
  plugin: string;
  options: unknown;
}

export interface StorageConfiguration {
  events?: StoragePluginConfiguration;
  files?: StoragePluginConfiguration;
  links?: StoragePluginConfiguration | "useEventStorage";
}

export interface EventQuery {
  schemaId?: string | string[];
  producer?: string | string[];
  timeRange?: {
    start?: string;
    end?: string;
  };
  links?: {
    type?: string;
    targetEvent?: EventId;
  }[];
  payload?: Record<string, unknown>;
  limit?: number;
  offset?: number;
  sort?: {
    field: string;
    direction: "asc" | "desc";
  }[];
}

export interface EventCreationOptions {
  producer?: string;
  files?: EventFile[];
  links?: EventLink[];
  metadata?: Record<string, unknown>;
}

export interface CoreSystem {
  config: CoreConfig;
  logger: Logger;
  validator: Validator;
  registerPlugin(plugin: Plugin): Promise<void>;
  getPlugin(name: string): Promise<Plugin | undefined>;
  executeHook(
    name: string,
    context: Partial<HookContext>,
  ): Promise<HookResult[]>;
  createEvent(
    schema: Record<string, unknown>,
    payload: Record<string, unknown>,
    options?: EventCreationOptions,
  ): Promise<Event>;
  getEvent(id: EventId): Promise<Event | null>;
  queryEvents(query: EventQuery): Promise<Event[]>;
}
