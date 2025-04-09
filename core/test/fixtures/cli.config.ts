const config = {
  storage: {
    events: {
      plugin: "memory-storage",
      options: {},
    },
    files: {
      plugin: "memory-storage",
      options: {},
    },
    links: "useEventStorage",
  },
  plugins: {
    sources: ["../../../plugins"],
  },
};

export default config;
