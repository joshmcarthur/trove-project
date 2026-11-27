# Contributing

Read the [README](https://github.com/joshmcarthur/trove/blob/main/README.md)
for uv, ruff, pytest, factory-boy, and `uv run zensical`. Contributor and
agent instructions live there. Do not add `AGENTS.md`.

1. Read the [spec](spec.md). Version 1 is the weekend build only.
2. Open the GitHub issue you are implementing. Issues on milestone `v1` are
   the work queue.
3. One issue per branch. Do not implement roadmap items.
4. Follow the README file layout. Plugins read through the query layer and
   write through the recorder only.

Preview the docs site with `uv run zensical serve`. CI on pull requests to
`main` runs `uv sync --frozen --group dev`, then ruff and pytest. Push to
`main` builds this site with `uv run zensical build` and deploys GitHub
Pages.

PR titles are Conventional Commits (`feat:`, `fix:`, `docs:`).
