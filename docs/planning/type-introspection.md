---
title: Type introspection
parent: Planning
nav_order: 15
---

# Type introspection

**Status:** Supported\
**Milestone:** 4\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/types`, `internal/modules`, `modules/type-catalog`

## Goal

Expose the local type catalog for listing, describing, exporting, and validating
TTD schemas via CLI and MCP. Validation checks TTD envelope and JTD
well-formedness only (`ParseTypeDefinition` + `Compile`); payload validation
against registered types remains at emit time.

## Built-in module

`type-catalog` is bundled like `mcp-query`. It declares:

- CLI command `types` with subcommands `list`, `describe`, `export`, `validate`
- MCP tools `list_types`, `describe_type`, `export_type`, `validate_type_schema`

## Core RPCs

Modules reach the catalog through `CoreServices`:

| RPC | Purpose |
|-----|---------|
| `ListTypes` | Summaries for all registered types (optional `source_filter`) |
| `GetType` | Summary + JTD `definition` for one URI |
| `ExportType` | Canonical TTD bytes from blob store |
| `ValidateTypeDefinition` | Parse and compile arbitrary TTD JSON |

## CLI

```bash
trove -config trove.toml types list [--json] [--source <filter>]
trove -config trove.toml types describe <uri> [--json]
trove -config trove.toml types export <uri> [-o <file>]
trove -config trove.toml types validate [--file <path>]   # stdin when omitted
```

## MCP

| Tool | Arguments | Result |
|------|-----------|--------|
| `list_types` | optional `source` | JSON array of type summaries |
| `describe_type` | `uri` | summary + `definition` |
| `export_type` | `uri` | canonical TTD JSON + `schema_ref` |
| `validate_type_schema` | `schema` (JSON) | `{valid, uri?, error?}` |

Tools are registered via `mcp-query` at `POST /mcp` like other module tools.

## Acceptance criteria

- [x] Catalog introspection RPCs on `CoreServices`
- [x] Bundled `type-catalog` module with CLI + MCP surfaces
- [x] `types list` shows builtins from `types/builtin/`
- [x] `types export` returns blob-stored canonical TTD bytes
- [x] `types validate` and `validate_type_schema` accept raw TTD JSON
- [x] Concept and roadmap updated

## See also

- [Type catalog](./type-catalog.md)
- [CLI commands](./cli-commands.md)
- [MCP tool registration](./mcp-tools.md)
