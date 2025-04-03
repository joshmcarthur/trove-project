import {
  assertEquals,
  assertRejects,
} from "https://deno.land/std/assert/mod.ts";
import { StorageManager } from "../storage.ts";
import { TestLogger } from "./utils.ts";
import { PluginSystem } from "../plugins.ts";
import { HookSystem } from "../hooks.ts";
import {
  CoreSystem,
  Event,
  EventFile,
  EventStorage,
  FileStorage,
  Plugin,
} from "../types.ts";

// Mock plugins for testing
const mockEventStorage: Plugin & EventStorage = {
  name: "mock-event-storage",
  version: "1.0.0",
  capabilities: ["storage:events"],
  initialize: () => Promise.resolve(),
  saveEvent: (event: Event) => Promise.resolve(event),
  getEvent: () => Promise.resolve(null),
  queryEvents: () => Promise.resolve([]),
};

const mockFileStorage: Plugin & FileStorage = {
  name: "mock-file-storage",
  version: "1.0.0",
  capabilities: ["storage:files"],
  initialize: () => Promise.resolve(),
  saveFile: (file: EventFile) => Promise.resolve(file.id),
  getFile: () => Promise.resolve(null),
  getFileData: () => Promise.resolve(new Uint8Array()),
};

Deno.test("StorageManager", async (t) => {
  await t.step("initialization", async (t) => {
    await t.step("requires event storage configuration", async () => {
      const logger = new TestLogger();
      const hooks = new HookSystem(logger);
      const plugins = new PluginSystem(
        {} as unknown as CoreSystem,
        hooks,
        logger,
      );
      const storage = new StorageManager(plugins, logger);
      plugins.loadPlugin(mockEventStorage);

      await assertRejects(
        () => storage.initialize({}),
        Error,
        "Event storage configuration is required",
      );
    });

    await t.step("loads event storage plugin", async () => {
      const logger = new TestLogger();
      const hooks = new HookSystem(logger);
      const plugins = new PluginSystem(
        {} as unknown as CoreSystem,
        hooks,
        logger,
      );
      await plugins.loadPlugin(mockEventStorage);

      const storage = new StorageManager(plugins, logger);
      await storage.initialize({
        events: {
          plugin: "mock-event-storage",
          options: {},
        },
      });

      // Verify plugin was loaded by trying to save an event
      const event: Event = {
        id: { id: "test" },
        createdAt: new Date().toISOString(),
        producer: "test",
        schema: { id: "test", version: "1.0" },
        payload: {},
        files: [],
        links: [],
      };

      const savedEvent = await storage.saveEvent(event);
      assertEquals(savedEvent, event);
    });
  });

  await t.step("file handling", async (t) => {
    await t.step("saves files before saving event", async () => {
      const logger = new TestLogger();
      const hooks = new HookSystem(logger);
      const plugins = new PluginSystem(
        {} as unknown as CoreSystem,
        hooks,
        logger,
      );

      // Load both storage plugins
      await plugins.loadPlugin(mockEventStorage);
      await plugins.loadPlugin(mockFileStorage);

      const storage = new StorageManager(plugins, logger);
      await storage.initialize({
        events: {
          plugin: "mock-event-storage",
          options: {},
        },
        files: {
          plugin: "mock-file-storage",
          options: {},
        },
      });

      const event: Event = {
        id: { id: "test" },
        createdAt: new Date().toISOString(),
        producer: "test",
        schema: { id: "test", version: "1.0" },
        payload: {},
        files: [{
          id: "file-id",
          contentType: "text/plain",
          size: 4,
          data: "test",
        }],
        links: [],
      };

      const savedEvent = await storage.saveEvent(event);
      assertEquals(savedEvent.files[0].id, "file-id");
    });
  });

  await t.step("error handling", async (t) => {
    await t.step("handles plugin not found", async () => {
      const logger = new TestLogger();
      const hooks = new HookSystem(logger);
      const plugins = new PluginSystem(
        {} as unknown as CoreSystem,
        hooks,
        logger,
      );
      const storage = new StorageManager(plugins, logger);

      await assertRejects(
        () =>
          storage.initialize({
            events: {
              plugin: "non-existent",
              options: {},
            },
          }),
        Error,
        "Plugin with required capabilities storage:events not found: non-existent",
      );
    });

    await t.step("handles missing capability", async () => {
      const logger = new TestLogger();
      const hooks = new HookSystem(logger);
      const plugins = new PluginSystem(
        {} as unknown as CoreSystem,
        hooks,
        logger,
      );

      // Load file storage plugin but try to use it for events
      await plugins.loadPlugin(mockFileStorage);

      const storage = new StorageManager(plugins, logger);
      await assertRejects(
        () =>
          storage.initialize({
            events: {
              plugin: "mock-file-storage",
              options: {},
            },
          }),
        Error,
        "Plugin with required capabilities storage:events not found: mock-file-storage",
      );
    });
  });
});
