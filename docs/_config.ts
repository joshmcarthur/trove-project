import lume from "lume/mod.ts";
import lumocs from "lumocs/mod.ts";
import mermaid from "lume_mermaid/mod.ts";

const site = lume();

site.use(lumocs());
site.use(mermaid({
  theme: "default",
  config: {
    startOnLoad: true,
    theme: "base",
    themeVariables: {
      primaryColor: "#e8f4fc",
      primaryTextColor: "#1a2a3a",
      primaryBorderColor: "#3d8fd1",
      lineColor: "#5a6a7a",
      fontFamily: "system-ui, sans-serif",
    },
  },
}));

site.copy("favicon.svg");

site.ignore((path) => path.includes("/superpowers/"));

export default site;
