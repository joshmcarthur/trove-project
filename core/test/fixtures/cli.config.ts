import { CoreConfig } from "../../types.ts";

const config: CoreConfig = {
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
    directories: ["plugins"],
  },
};

export default config;
