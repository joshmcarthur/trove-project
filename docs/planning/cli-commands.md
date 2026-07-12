---
title: CLI commands
parent: Planning
nav_order: 14
---

# CLI command registration

**Status:** Supported\
**Milestone:** 4\
**Spec:** [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/modules`, `cmd/trove`

## Goal

Let modules contribute top-level CLI commands the same way they contribute HTTP
routes and MCP tools. Core flags (`-version`, `-config`) are reserved and never
replaced by module commands.

## Manifest

```toml
[[cli.commands]]
name = "types"
description = "List, export, and validate Trove type schemas"
```

- Command names must be unique across all discovered modules (startup fails on duplicates)
- Reserved names: `version`, `config`, `help`, `init`
- Module subprocess implements `trovemodule.CLIHandler`

## Invocation

```bash
trove init
trove init --dir /path/to/data
trove -config trove.toml types list
trove -config trove.toml types validate --file ./my-type.ttd.json
```

`init` writes a default `trove.toml` and `blobs/` directory in the working
directory (or `--dir`). It does not require `-config`.

When the first positional argument after flags matches a collected CLI command,
the host builds the type catalog, starts only the owning module subprocess,
invokes `CLIModule.RunCommand`, prints stdout/stderr, and exits. Otherwise the
host falls through to the long-running daemon path.

## RPC surface

- `CLIModule.RunCommand` — implemented by CLI-providing modules
- `CoreServices` — unchanged for CLI dispatch; modules use existing Core APIs

## Acceptance criteria

- [x] `[[cli.commands]]` parsed and validated from manifest
- [x] Duplicate command names and reserved names rejected at startup
- [x] `trove -config <path> <command> ...` dispatches to the owning module
- [x] Core `-version`, `init`, and `-config` daemon behavior unchanged when no module command matches

## See also

- [Type introspection](./type-introspection.md)
- [MCP tool registration](./mcp-tools.md)
