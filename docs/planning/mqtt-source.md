---
title: MQTT source
parent: Planning
nav_order: 5
---

# MQTT source

**Status:** Planned\
**Milestone:** 2\
**Spec:** [Sources §6](../spec.md#6-sources), [Build order §11.2](../spec.md#11-build-order--validation-plan)\
**Package:** external source module

## Goal

Subscribe to configured MQTT topics on Mosquitto; wrap each message as an event.
Covers Meshtastic (MQTT-bridged) and ESPHome sensor traffic.

## Interfaces

Source module — `Emit(event)` per message. Suggested event type:
`mqtt.<topic-with-dots>.received` or configured mapping in module config.

## Implementation notes

- Module config: broker URL, client id, topics, QoS, credentials
- Payload: raw message bytes as JSON string or parsed JSON when valid
- `source`: topic or configured device id
- Reconnect with backoff

## Acceptance criteria

- [ ] Subscribes to configured topics
- [ ] Each message appends one journal event
- [ ] Survives broker disconnect/reconnect
- [ ] Healthcheck reports subscription status

## Dependencies

- **Blocks:** hardware-adjacent validation (Meshtastic path)
- **Blocked by:** journal, module runtime

## Open questions

- Topic → `type` mapping convention
