# Trove Collector — System Specification

Nov 27, 2026 · @Josh

## Purpose and scope

Trove is a self-hosted, single-user collector: send it anything and it catalogues it into typed, linked, searchable items that you can correct and that never lose their originals. It replaces Google Photos as the place for things worth keeping: colours, design and app screenshots, curated photos (including a few family shots) and links.

It is AI-first: an agent fills code-declared types from what you upload, and deterministic processors (such as palette extraction) run alongside. A self-developing registry, where the agent proposes types for you to ratify, is on the roadmap.

Its shape follows Home Assistant: one core process, small JSON configuration files beside the database, integration-style plugins, and add-ons under supervision. The surface: ingest files and metadata, store typed and linked items, and view them on a wall. MCP and Telegram follow as add-ons.

**Goals**

- **A weekend build, then tweaks.** Version 1 is sized to build in a weekend. Everything else is roadmap, added only when the live test earns it.
- **No maintenance.** Few dependencies, an LTS framework, nothing that must be upgraded to keep working, and one-command backups.
- **Personal software.** One person, one install, no platform ambitions.
- **A pathway to native apps.** Ingest accepts files from any authenticated client, starting with an iOS Shortcut and later a native share extension, following Home Assistant's companion-app model.

**Non-goals**

- Multi-user or collaborative use; one person, one install.
- Bulk photo management. Family photos stay in a photo library; only curated shots come here.
- Multi-node sync or alternative deployment targets beyond one home server.
- A REST API for its own sake. Machine interfaces exist only where an integration needs them (ingest, the wall, MCP, Telegram).

## Design principles

1. **Facts are append-only; state is rebuildable.** Facts are never updated or deleted, except by an explicit purge. State tables hold no information of their own and can always be rebuilt from facts.
2. **Originals are never modified.** Everything learned about an item is layered on top as claims and links.
3. **Every write happens inside an activity.** Provenance (who or what, when, with which model and prompt, at what cost) cannot be skipped.
4. **Rank, then recency.** Conflicting values resolve by rank (preferred, normal, deprecated) and only then by newest. A human correction outranks AI output.
5. **Captures are dumb; meaning comes from processors.** Ingest stores what arrived. Typed meaning is added afterwards by processors and people.
6. **Derived data is re-derivable.** Processor output can be regenerated; only originals, human claims and decisions are irreplaceable.
7. **One read layer, one write layer.** All reads go through one query layer that knows the resolution rules; all writes go through the activity API. Plugins never touch models directly.
8. **Columns where the shape is fixed, JSON where the type system is open.**
9. **Boring technology.** Django built-ins first, then small packages, then simple Python, then infrastructure. Each dependency must solve a concrete problem.
10. **The table count follows the requirements.** Tables are split by identity, lifecycle and growth, not to hit a target count.
11. **Follow Home Assistant's shape.** When in doubt about storage, auth, plugins or supervision, do what HA does at personal scale.
12. **Version 1 first.** A feature joins version 1 only if the weekend build cannot work without it.

## Home Assistant model

Trove is close to a Home Assistant with a different data model: HA's entities, states and events become items, claims and activities, and its storage, auth, plugin and supervision patterns carry over directly.

| Concern | Home Assistant | Trove |
| --- | --- | --- |
| History | SQLite recorder: states and events, purged after a retention period | SQLite: items, blobs, claims, links, activities. Never purged; this is the collection. |
| Configuration | JSON files under `.storage/` (config entries, registries, auth), plus YAML | JSON files under `.storage/`: core settings, plugin enablement and config entries, data-defined types later. Human-editable, diffable, backed up as files. |
| Auth | Its own OAuth-style flows, refresh tokens, long-lived tokens for scripts | Its own OAuth 2.1 server, passkey owner sign-in, long-lived tokens for Shortcuts |
| Mobile apps | Companion app signs in, registers the device (`mobile_app`), posts to a per-device webhook | Shortcut now; later a native app that signs in, registers, and posts files to ingest |
| Plugins | Integrations: a folder with `manifest.json` and a setup function; `custom_components` for your own | A folder with `manifest.json` and a setup function; built-in plugins plus a `plugins/` folder for your own |
| Process model | One core process running its own background work | One core process serving the web and running jobs in a thread pool |
| Supervision | Supervisor manages add-ons, backups and updates | supervisord runs the core and add-ons (MCP, Telegram); `manage.py backup` makes one tarball |

One difference matters. HA's companion apps share only text and URLs, with no file support, which is the gap Trove's ingest fills.

## Data model

Five append-only fact tables: `items`, `blobs`, `claims`, `links` and `activities`. Items are stable identities, like Home Assistant entities. Everything known about an item is a claim or a link, and every claim, link and item records the activity that produced it. The type registry is not a table; it is built at startup.

| Table | Kind | Holds | Key fields |
| --- | --- | --- | --- |
| `items` | Fact | Stable identity of anything: originals, emitted things (colours, regions, bulbs), type proposals | id, type, activity, created |
| `blobs` | Fact | Content-addressed files | sha256 (fixity), size, content type |
| `claims` | Fact | Every field value, from processors and from people | item, path, value (JSON), rank, activity, created |
| `links` | Fact | Relations between items, including `derived_from` | source item, target item, rel, activity, retracts (optional) |
| `activities` | Fact | Everything that happened: ingest, derive, edit, decide, purge | kind, agent, item(s), parameters (model, prompt hash, decision), cost, outcome, error, started, ended |

**Items.** An original is an item created by an ingest activity. An emitted thing is an item created by a derive activity and linked to its source with `derived_from`. A colour is one item per colour per image, keyed by hex. A region (a crop, a bulb in a screenshot) is an emitted item with a `$box` claim: a pixel rectangle on the source, for example `{"x":120,"y":40,"w":300,"h":180}`.

**Crops.** A processor that finds a region emits the crop as its own item, linked to the original with `derived_from`. The crop's pixels are a `$blob` claim, so it is a normal image that other processors (palette, the agent) can consume. Its `$box` claim records where on the original it came from: re-derives use it to recognise the same region by overlap, and the wall uses it to draw the region on the original.

**Claims.** A claim is one value at one path on one item. A blob is referenced from a claim (for example `$blob`), so a re-crop is a new claim with a new blob on the same item, and a moved region is a newer `$box` claim.

**File metadata.** Format-specific facts (an image's width and height, a PDF's page count, a recording's duration) are claims on the item, written by an extraction step at ingest. `blobs` holds only what is true of any file, so documents and other file types fit without schema changes.

- **Rank:** preferred (your edits and the metadata you supply at capture), normal (processor and AI output, MCP writes), deprecated (hidden).

**Links.** Links are asserted and retracted, whoever makes them: an unlink is a new link row whose `retracts` points at the original. A processor follows the same rule as a person: if a re-derive no longer supports a link it made earlier, it writes a retraction.

**Activities.** Activities are never purged. They are the derivation state ("has processor P at version 3 handled this item?"), the cost ledger, the failure record and the decision history. A purge's activity is the tombstone. Agents are processors with versions (`palette@2`, `agent@5`), the user, or a named client (`mcp:claude-desktop`, `telegram`).

**Types.** Version 1 builds the registry at startup from code-declared types. Data-defined types, stored in `.storage/` and rebuilt from decide activities, come later (see Roadmap).

**Outside the model:** configuration lives in `.storage/` JSON files. `django-oauth-toolkit` keeps OAuth clients and tokens in its own tables. Later, plugin-owned indexes (such as embeddings) are rebuildable state.

The model's lineage: W3C PROV and PREMIS (activities and agents), Wikidata (rank) and Datomic (append-only facts with transaction provenance).

## Read and write rules

All writes go through the activity API and all reads through one query layer, so the resolution rules exist in exactly one place.

**Writing**

```python
with activity(kind="edit", agent=user) as act:
    act.claim(item, "base", "E27", motivation="correcting", rank="preferred")
    act.link(bulb, lamp, rel="fits")
```

- An activity opens a transaction, records its agent and parameters, and closes with an outcome and cost.
- Append-only is enforced twice: model `save()` refuses updates, and database triggers reject `UPDATE` and `DELETE` on fact tables. Triggers are written for SQLite only; any other backend would need its own.
- Purge, when it arrives, will be the only code path allowed to bypass the triggers, dropping and recreating them inside its own transaction.

**Reading (`queries.py`)**

- **Current value of a path:** highest rank wins; within a rank, newest wins. Deprecated claims are hidden. This applies across authors, so a newer AI claim never overrides a preferred human one.
- **Stale AI fields:** an AI claim for a path stays current until a newer claim replaces it, even if later runs stop producing that path (accepted risk).
- **Current links:** all asserted links minus retracted ones, with no difference between human and processor links.
- **Reserved types:** proposal items and other `trove.*` types are excluded from collection views unless asked for.
- **Provenance:** every value and link can be traced to its activity and agent, giving "who set this, when, and how" in one query.
- Plugins read only through this layer, by convention.

**Purge (roadmap)**

Purging an item deletes its rows across items, claims and links, and walks `derived_from` to remove everything emitted from it. Its blob is deleted only if no remaining claim references it. Items sharing the same blob are purged together. The purge activity, recording what was removed (without content), is the tombstone. Backups age out on rotation; purge does not rewrite them.

**Upcasting (roadmap)**

Only irreplaceable values need upcasters: human claim values and activity parameters, when their type version changes. Processor output is re-derived instead.

## Type registry

Version 1 has code types only: Pydantic models declared by plugins. Data types and the proposal lifecycle below are roadmap, built only if the live test passes. When they arrive, both sources look identical downstream.

- **Code types** are Pydantic models in plugins. They are the steering mechanism: declare a type and the agent starts filling it.
- **Data types** (roadmap) are constrained field specs stored in `.storage/`, built into Pydantic models at runtime. Any shape the store can represent is allowed, including nested objects and lists. References to other items become links, not fields.
- **Schemas are never hand-written.** JSON Schema for the model's structured output is generated from the Pydantic model; the same model validates the result.
- **`describe()`** produces each type's name, description and generated schema for prompts.

**Lifecycle (roadmap)**

1. The scheduled whole-collection pass emits a proposal as an item of type `trove.proposal`: a type name, its fields and an example filled from a real item. Per-item extraction never proposes.
2. Values extracted for a proposed type are stored as provisional claims and shown greyed on the wall.
3. You decide through the API (an MCP tool or manage.py decide): ratify, rename, merge into an existing type, or reject. Each decision is a `decide` activity. Rejections are remembered so the same type isn't proposed again.
4. On ratify, provisional values are superseded by a backfill across the whole collection. On reject, they are hidden but kept.
5. Editing a ratified type creates a new version and re-derives that type.

The agent always sees ratified types and pending proposals, to avoid proposing near-duplicates.

## Derivation and jobs

Processors are declared by plugins and run inside the core process on a thread pool, the way Home Assistant runs its background work. There is no queue table and no worker process: derivation state lives in `activities`, and reconcile makes the work durable.

**Processor contract**

```python
Processor(name, version,
          consumes="image.*",            # item types it reacts to
          provides=["colour"] | REGISTRY,  # REGISTRY = any registered type
          attempts=3, run=fn)             # run(item, ctx) -> Result(items, claims, links, cost)
```

- A processor emits new items, claims and links through its result; the derive task writes them inside a `derive` activity whose agent is `name@version`.
- Exactly one processor, the agent, may provide `REGISTRY`.

**How jobs run**

- **Submit:** when an item is created, matching processors are submitted to the core's thread pool.
- **Reconcile:** at startup and whenever a processor's version changes, find items matching its `consumes` with no successful `derive` activity at the current version, and submit them. A crash or restart loses nothing; reconcile finds the work again.
- **Retries:** transient errors (429, 5xx) retry in-process with backoff, honouring `Retry-After`. After the last attempt, or on a permanent error, the activity ends with outcome `failed`, which the admin shows.
- **Timeouts:** each processor sets timeouts on its own clients; a hung call ties up one pool thread, nothing more.
- **Priority:** new captures run before reconcile backfill.
- **Spend:** version 1 relies on the model provider's own spend limit. Per-lane budgets are roadmap.
- **Loop safety:** a cycle check over processors' `consumes` and `provides` at startup refuses to start on a loop.
- **Plugin isolation:** each plugin loads independently; a broken built-in plugin is disabled and listed in the admin, not fatal.

## AI extraction

In version 1, one cloud-model agent fills the registered types for each new item in a single call: it is given the item and each type's generated schema, and its results are written as claims in one `derive` activity, with the model, prompt hash and cost recorded. The model is reached through an OpenAI-compatible API, so changing models is a configuration change. Everything goes to the cloud model, including curated family shots (accepted risk).

The rest of this section is roadmap: two-stage extraction, regions, learning from corrections and proposals.

**Two-stage extraction (roadmap, once the registry grows)**

1. **Select:** a cheap call looks at the item and returns candidate types plus a short text description of it.
2. **Retrieve:** the item's image embedding and the description's text embedding search two `sqlite-vec` indexes; results are merged by reciprocal rank fusion to find similar items carrying your corrections.
3. **Fill:** a second call fills only the candidate types, given their generated schemas, those similar corrections as examples, and the item's existing regions and keys.
4. **Write:** results become items, claims and links in one `derive` activity, with the model, prompt hash and cost recorded.

**Regions and identity (roadmap)**

- Several items of one type may come from one source (three bulbs in one screenshot). Each is a region item with a box claim and, where useful, a crop blob kept as a claim.
- On re-derive, a new box is matched to an existing region by overlap (intersection over union above a threshold, starting at 0.5). A match keeps the item's identity and records the new box as a `$box` claim; no match creates a new item.
- Matching lives in the core, because identity rules are core rules. Uniqueness of emitted items is enforced in application code, since the source is a link.

**Learning from corrections (roadmap)**

A correction is a preferred claim you make on a path a processor had already filled. The query layer finds corrections by that shape, so they form a queryable history with no extra marker. Similar past corrections are retrieved by embedding search and fed into extraction prompts. A re-derive that still disagrees with you cannot override you, because rank beats recency; the conflict is visible on the wall.

**Proposals (roadmap)**

Only the scheduled whole-collection pass proposes types, from clusters of items no type covers. See the type registry lifecycle above.

**Deterministic processors run alongside.** The palette extractor produces exact colours; the agent is never asked for colour values.

## Core

The core is one Django app that owns the data model and every rule about it, laid out in small files per the django-for-joshmcarthur conventions. It has no knowledge of AI.

```
config/                 settings, urls, asgi
trove/
  models/               item, blob, claim, link, activity (one file each)
  queries.py            current values (rank, then newest), live links
  activity.py           the only write path
  registry.py           code types, describe()
  jobs.py               thread pool, reconcile, retries, cycle check
  storage.py            .storage/ JSON: core settings, plugin config entries
  auth.py               setup code, owner passkey, OAuth scopes, client-to-agent naming
  plugins.py            loader: manifest.json, setup(), enable/disable
  backup.py             manage.py backup: database, blobs and .storage/ in one tarball
  processes.py          renders supervisord config
  admin.py              Django Admin: AI output, failures, plugins, tokens
plugins/
  ingest/               core
  wall/ palette/ agent/                    built-in, version 1
  mcp/ telegram/ embeddings/ proposals/    built-in, roadmap
.storage/
  core.json             model endpoint, settings
  plugins.json          enablement and per-plugin config entries
```

**Plugin contract.** As in a Home Assistant integration, each plugin is a folder with a `manifest.json` (domain, name, version, kind, dependencies, Python requirements) and a `setup(core, entry)` function. `setup` receives the plugin's config entry from `.storage/plugins.json` and declares:

- types (Pydantic models)
- processors
- views, mounted under `/<plugin>/`
- processes, for supervisord
- templates and static files
- rebuildable indexes it owns (never truth)

Plugins read through `queries.py` and write through `activity.py` only. Each also declares whether it is core or built-in, and which plugins it needs. They are discovered from the packaged set, a `plugins/` folder (yours) and entry points (shared). Each loads independently: a broken built-in plugin is disabled and listed in the admin; a broken core plugin stops startup.

**Estimated size:** version 1's core is roughly 500 to 800 lines of Python, most of it in `jobs.py`, `queries.py` and `auth.py`. A spike of the storage and job core came to about 190 lines.

## Core and built-in plugins

Core plugins must be present: Trove will not start without them. Built-in plugins ship with Trove and are enabled by default, but any of them can be turned off in `.storage/plugins.json`. Ingest is the only core plugin; nothing else is needed to capture and keep things.

| Plugin | Kind | Role | Declares |
| --- | --- | --- | --- |
| ingest | Core | `POST` a file plus metadata. Creates a blob, an item and claims inside an `ingest` activity whose agent is the client's name. Metadata you supply (title, notes) is a preferred claim. | Views, original item types |
| wall | Built-in, on | A JSON read API and a small SPA to view item state: grid, current values, region boxes, provisional values greyed, provenance timeline. Not a control plane. | Views, static files |
| mcp | Built-in, roadmap | Fixed toolset: `recent`, `search`, `get` (with provenance), `claim`, `link`, `decide`. Writes are attributed to the client. | A process |
| palette | Built-in, on | Deterministic: consumes images, emits one colour item per colour, keyed by hex. | A processor, the `colour` type |
| agent | Built-in, on | Two-stage extraction filling the registry; emits items, claims, region items with boxes, and crop blobs. Off leaves deterministic processors only. | A processor providing `REGISTRY` |
| proposals | Built-in, roadmap; needs agent | The type discovery pass: scheduled, whole-collection, emitting `trove.proposal` items. | A scheduled processor |
| embeddings | Built-in, roadmap | Image and text embeddings into two `sqlite-vec` indexes it owns. When off, the agent runs without correction examples. | A processor, an index |
| telegram (later) | Built-in, roadmap | Another ingest channel, long-polling. | A process |

Turning a plugin off unmounts its views, stops its processors and processes, and drops it from reconcile. Its facts stay; its indexes can be deleted and rebuilt. Turning it back on lets reconcile catch up. A plugin whose dependency is off is disabled with it. Operations (failures, budgets, plugin status, tokens) live in the Django admin and `manage.py status`, not the wall.

The first build ships ingest, wall, palette and agent with a handful of hand-written types; see Build order.

## Interfaces and auth

Trove is its own OAuth 2.1 authorization server, using `django-oauth-toolkit`. Every interface presents a scoped bearer access token; you approve clients by signing in with a passkey, and there is no password. The server is reached over Tailscale.

**First run.** On first start the server prints a one-time setup code to its console, and `manage.py setup_code` shows it again. The `/setup` page asks for that code, then registers your passkey as the owner credential. Only someone with shell access to the server can claim it, and the code dies once used.

| Interface | Client | Grant | Notes |
| --- | --- | --- | --- |
| Wall | SPA in the browser | Authorization code + PKCE, public client | Access token held in memory, rotating refresh token. No cookies, so no CSRF. Scope `read:all`. |
| MCP | Local and cloud MCP clients (roadmap) | Authorization code + PKCE | The flow the MCP authorization spec expects: the client opens a browser, you approve with your passkey. Scopes such as `read:all`, `write:claim`, `write:link`, `write:decide`. |
| Ingest | iOS share-sheet Shortcut (v0), later a native share extension; curl | Long-lived token minted with `manage.py token create` or in the admin | Shortcuts cannot run a browser flow, so this mirrors Home Assistant's long-lived tokens. Scope `write:capture` only. Device authorization grant is the upgrade path. |
| Telegram (later) | Telegram bot | Bot token plus an allowed chat ID | Not OAuth; an ingest channel. |

Every token belongs to a registered client, and the client's name becomes the agent recorded on its activities (for example `mcp:claude-desktop`). Tokens are listed and revoked in the admin. Tokens are labelled local or cloud at approval; Trove cannot verify which a client really is.

With everything sent to a cloud model, the privacy gate (local classifier, `privacy.cleared`, cleared-only MCP scopes) has been removed. Scopes now control what a client may do, not what it may see.

## Processes and deployment

Like Home Assistant, Trove is one core process with add-ons beside it. The core serves the web and runs jobs in its own thread pool. Plugins may declare add-on processes; `manage.py processes` renders the core and add-ons into supervisord config, and supervisord runs and restarts them on one home server.

| Process | Declared by | Command |
| --- | --- | --- |
| web | Core | ASGI server running Django and the job thread pool |
| mcp | mcp plugin | The MCP SDK's own server, on its own port (roadmap) |
| telegram (later) | telegram plugin | Long-polling bot |

- **MCP runs as its own process.** The MCP SDK changes quickly (2.x renamed `FastMCP` to `MCPServer`), so it is kept behind a process boundary and pinned tightly.
- **One restart policy.** supervisord gives automatic restarts, a log per process, and `supervisorctl restart telegram` to bounce one process.
- **Development** on macOS uses the same rendered process list (via `honcho` or supervisord).
- **Storage and backup.** One SQLite file in WAL mode, the blob directory and `.storage/`. `manage.py backup` writes one tarball: the database (via SQLite's backup API), then the blobs, then `.storage/`. Content-addressed blobs make that order safe. Restore is unpacking the tarball.
- **Python:** any current CPython, managed with uv. SQLite extension loading only matters once embeddings (`sqlite-vec`) arrive.

## Stack and dependencies

Few, stable dependencies on an LTS framework, chosen so that nothing needs upgrading to keep working. Version 1 uses Django 5.2 LTS rather than the django-for-joshmcarthur default of Django 6: running jobs in-process removes the need for the Tasks framework, and LTS means security fixes without feature churn. Pin everything; versions as of November 2026.

| Dependency | Why | Note |
| --- | --- | --- |
| Django 5.2 LTS | Framework | Supported until April 2028; move to the next LTS when it lands |
| SQLite (WAL) | Storage | The only backend |
| `django-allauth` | Owner passkey sign-in; the approval step of every OAuth grant | Skill default |
| `django-oauth-toolkit` | OAuth 2.1 authorization server: PKCE grants, scopes, token storage and revocation | Mature and widely used; the standard Django choice |
| Preact + htm | The wall SPA, with no build step | Vendored ES modules, a few kB |
| Pydantic | Type models, generated JSON Schema for structured output, validation | Needed for LLM structured output |
| `openai` Python client | The agent's model calls, against any OpenAI-compatible endpoint | Changing models or providers is configuration |
| Pillow (+ `pillow-heif`) | Palette extraction, file metadata | `pillow-heif` for iPhone HEIC uploads |
| uvicorn | ASGI server for the core process |  |
| supervisord | Process supervision | Old, stable, Python |

**Roadmap dependencies:** `sqlite-vec` (embeddings, pinned behind one module), the MCP Python SDK (pinned tightly; it churns).

Not used: a task queue or worker, Redis, Celery, Postgres, Docker by default, a frontend build pipeline, React, or a REST framework.

## Accepted risks

Two risks were accepted deliberately and should not be reopened without a reason.

| Risk | Consequence | Why accepted |
| --- | --- | --- |
| Cloud model for everything | Every item, including curated family shots, is sent to a cloud provider. | Better extraction quality; removes the privacy gate and its complexity. |
| Stale AI fields | A field a later run stops producing keeps its old AI value. | Simpler resolution rule; the wall shows each value's provenance and age. |

The earlier "human correction stomped by a re-derive" risk is resolved by rank and no longer applies.

## Version 1 and roadmap

Version 1 is a weekend build that can pass a two-week live test. Everything else waits until the test says it is worth building. Nothing is being built yet.

1. **Saturday morning, core:** the five tables with SQLite triggers, the activity API, `queries.py`, code-type registry, `.storage/` loader, plugin loader with manifests.
2. **Saturday afternoon, auth and ingest:** setup code, owner passkey, OAuth for the wall, long-lived tokens, and the ingest plugin with file metadata. The iOS Shortcut posts a file and gets a 201.
3. **Sunday morning, jobs and processors:** the thread pool with reconcile and retries, palette, and the agent filling a handful of hand-written types (for example a bulb, a reference shot, a link).
4. **Sunday afternoon, wall and ops:** the wall SPA, the admin, `manage.py backup`, `manage.py processes` and supervisord on the home server.
5. **Two-week live test.** Kill criterion: *after two weeks, would I miss it if it vanished?* If not, stop and fall back to a Google Photos album.

Auth is the largest piece of the weekend. If it runs long, the wall can start behind Tailscale with the owner session alone and gain OAuth later.

**Roadmap, roughly in order, each only when needed**

1. MCP add-on, with OAuth for MCP clients.
2. Regions and crops, with overlap-based identity.
3. Embeddings and learning from corrections.
4. Two-stage extraction, data types, proposals and whole-collection backfill.
5. Per-lane budgets, purge and upcasting.
6. Telegram add-on.
7. A native iOS app: sign in, register the device like HA's `mobile_app`, and post files from a share extension.
8. A plain-file mirror; SQLite's own `vec1` when it reaches 1.0.

**Open questions**

- Where the rewrite lives: the existing `trove-project` repository as a new major version, or a new repository.
- Whether OAuth tokens move from `django-oauth-toolkit`'s tables into `.storage/`, as Home Assistant keeps auth. The library expects database tables, so version 1 keeps them there.
- The exact MCP tool shapes and token scopes, when MCP arrives.

## Background and decision history

This design replaces the Go implementation of [Trove](https://github.com/joshmcarthur/trove-project), which had become a general "everything app" and never ran its planned two-week live test. The key reversals:

| Decision | Was | Now | Why |
| --- | --- | --- | --- |
| Language and framework | Go, go-plugin over gRPC | Python, Django 5.2 LTS, in-process plugins | Most processors are Python-native; the plugin runtime was about half the code and caused upload and blob-read gaps |
| Storage model | Event journal, revisions, folds, RFC 6902 transforms | Append-only facts plus rebuildable state | The collector is append-only by design; the fold layer solved mutations that no longer exist |
| Jobs | Custom watermark router | In-process thread pool, as in Home Assistant; state in activities | The router blocked on one failing processor; the domain state belongs in the data model |
| Schemas | JTD type catalog | Pydantic, generated JSON Schema | Maintained library; LLM structured output takes JSON Schema |
| Retrieval | Conversational via MCP | A visual wall first, MCP second | Browsing is how the collection gets used |
| Typing | Hand-written processors per type | AI fills a registry and proposes types | Follows what is actually saved; config types steer it |
| Conflicts | Newest wins | Rank, then newest | The archival norm; stops re-derives overriding corrections |

**Alternatives rejected:** Karakeep (strong, but tags only, with no typed fields or registry); a Starlette-only stack (1.0 only in March 2026, no LTS); a separate worker process on `django-ox` (in-process jobs with reconcile are simpler and need no queue); a single-table data model (it hid distinct lifecycles).

**Scope correction (November 2026):** a review against the goals (weekend build, no maintenance, personal software) found the full design was months of work. Version 1 was cut to the core, ingest, wall, palette and a simple agent; everything else became roadmap. The job queue, `django-ox` and Django 6 were dropped in favour of Home Assistant's in-process model on Django 5.2 LTS.

**Sources:** [django-ox](https://github.com/oxpull/django-ox) · [django-ox 1.9.0 release](https://github.com/oxpull/django-ox/releases/tag/v1.9.0) · [Starlette 1.0 and support policy](https://tuxcare.com/blog/starlette-0-x-end-of-life-and-what-asgi-teams-should-do-next/) · [MCP SDK: mounting in ASGI apps](https://py.sdk.modelcontextprotocol.io/run/asgi/) · [sqlite-vec releases](https://www.piwheels.org/project/sqlite-vec/) · [SQLite vec1](https://sqlite.org/forum/info/a09b103d9d41ed25a357a42aa22ca32a93d4a5fb9b10eb40ab3e14cde508e8f1?t=c) · [Karakeep](https://github.com/karakeep-app/karakeep)
