---
title: AT Protocol bridge (superseded)
parent: Planning
nav_order: 18
---

# AT Protocol bridge (superseded)

**Status:** Superseded\
**See:** [AT Protocol PDS module](./atproto-pds.md)

The original bridge plan targeted a non-federated translation layer (journal as
canonical store, no firehose). The preferred direction is now a **single-user
federated PDS module** that owns signing, DID, repo storage, and WebSocket
federation while optionally mirroring into the Trove journal for MCP search.

All new work should follow [atproto-pds.md](./atproto-pds.md).
