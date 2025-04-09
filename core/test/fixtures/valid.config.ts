
    export default {
      storage: { events: { plugin: "memory-storage", options: {} } },
      plugins: {
        sources: [
          "./relative_plugin.ts", // Relative path
          "https://example.com/remote_plugin.ts", // Absolute URL
          "/absolute_plugin.ts" // Absolute path
        ],
        config: { /* plugin specific config */ }
      }
    };
  