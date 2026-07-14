# Changelog

## Unreleased

### BREAKING CHANGES

* Rename journal **Event** to **Revision** across SQLite (`revisions` table), RPC
  (`AppendRevision`), and module SDK (`RevisionAppender` / `RevisionQuerier`).
* MCP is **records-only**: `get_record`, `search_records`, `list_incomplete_records`.
  Removed `get_event`, `search_events`, `get_events_by_type`, and `summarize_range`.
* HTTP ingest endpoint is `POST /records`; response field is `revision_id` (was `event_id`).
* Legacy databases with `events` tables migrate to `revisions` on `journal.Open`.

## [2.0.0](https://github.com/joshmcarthur/trove-project/compare/v1.0.1...v2.0.0) (2026-07-14)


### ⚠ BREAKING CHANGES

* revision rename ([#116](https://github.com/joshmcarthur/trove-project/issues/116))
* rename journal Event to Revision (code) ([#118](https://github.com/joshmcarthur/trove-project/issues/118))
* drop consumes_operations manifest routing ([#115](https://github.com/joshmcarthur/trove-project/issues/115))
* update spec and supersede deferred-capture ([#112](https://github.com/joshmcarthur/trove-project/issues/112))
* migrate telegram-source and remove capture-classifier ([#110](https://github.com/joshmcarthur/trove-project/issues/110))
* migrate mqtt-source to RecordWrite ([#105](https://github.com/joshmcarthur/trove-project/issues/105))
* replace HTTP ingest with POST /records ([#106](https://github.com/joshmcarthur/trove-project/issues/106))
* add record query RPC and MCP tools ([#108](https://github.com/joshmcarthur/trove-project/issues/108))
* add EmitRecord RPC and remove Emit ([#107](https://github.com/joshmcarthur/trove-project/issues/107))
* reshape journal event schema for apply/delete ([#103](https://github.com/joshmcarthur/trove-project/issues/103))

### Features

* add EmitRecord RPC and remove Emit ([#107](https://github.com/joshmcarthur/trove-project/issues/107)) ([2b15c1b](https://github.com/joshmcarthur/trove-project/commit/2b15c1b2e4d447f5d0f6bf038823aa006e7871ad))
* add record fold primitives ([#104](https://github.com/joshmcarthur/trove-project/issues/104)) ([38dd299](https://github.com/joshmcarthur/trove-project/commit/38dd299ef288d13e19a47f40cc485d65469da034))
* add record query RPC and MCP tools ([#108](https://github.com/joshmcarthur/trove-project/issues/108)) ([22563c2](https://github.com/joshmcarthur/trove-project/commit/22563c243e84f9d883a973b6836d5b901805cf3a))
* add trove init CLI subcommand ([#98](https://github.com/joshmcarthur/trove-project/issues/98)) ([e4f9f0b](https://github.com/joshmcarthur/trove-project/commit/e4f9f0ba572967a7e63e73a33bcbb5f674620a98))
* migrate mqtt-source to RecordWrite ([#105](https://github.com/joshmcarthur/trove-project/issues/105)) ([deef397](https://github.com/joshmcarthur/trove-project/commit/deef397b967f49cc014df30faf3be84bafafa70b))
* migrate telegram-source and remove capture-classifier ([#110](https://github.com/joshmcarthur/trove-project/issues/110)) ([10c1f2e](https://github.com/joshmcarthur/trove-project/commit/10c1f2e11271d5fed7834072c068ce9f9914bd9f))
* rename journal Event to Revision (code) ([#118](https://github.com/joshmcarthur/trove-project/issues/118)) ([2355627](https://github.com/joshmcarthur/trove-project/commit/2355627ca1358174a52e85d08fe14abb8917f1e6))
* replace HTTP ingest with POST /records ([#106](https://github.com/joshmcarthur/trove-project/issues/106)) ([417da45](https://github.com/joshmcarthur/trove-project/commit/417da456f27c6b5985de015aa1921b3b5f63c0df))
* reshape journal event schema for apply/delete ([#103](https://github.com/joshmcarthur/trove-project/issues/103)) ([ebe8229](https://github.com/joshmcarthur/trove-project/commit/ebe822950dcc38e44e0d178f02663421e43548a1))


### Documentation

* revision rename ([#116](https://github.com/joshmcarthur/trove-project/issues/116)) ([65332ed](https://github.com/joshmcarthur/trove-project/commit/65332ed1cbe7e92fda076a029acb8eed64caf76b))
* update spec and supersede deferred-capture ([#112](https://github.com/joshmcarthur/trove-project/issues/112)) ([0e7fe7d](https://github.com/joshmcarthur/trove-project/commit/0e7fe7d09deceea50b2a7933b1c00bff58e339b5))


### Refactors

* drop consumes_operations manifest routing ([#115](https://github.com/joshmcarthur/trove-project/issues/115)) ([3b8cdfa](https://github.com/joshmcarthur/trove-project/commit/3b8cdfad681a89e0a28554596e3cc4dfcc46c216))

## [1.0.1](https://github.com/joshmcarthur/trove-project/compare/v1.0.0...v1.0.1) (2026-07-12)


### Bug Fixes

* goreleaser stable release brew and windows binary names ([#93](https://github.com/joshmcarthur/trove-project/issues/93)) ([3b0de10](https://github.com/joshmcarthur/trove-project/commit/3b0de1095fcd7e7f9cc83eafb0f6c303bd3046c2))

## 1.0.0 (2026-07-12)


### Features

* add release automation with GoReleaser and release-please ([#89](https://github.com/joshmcarthur/trove-project/issues/89)) ([1d21303](https://github.com/joshmcarthur/trove-project/commit/1d21303e534df8e739021650e9c23b7d068d5818))
* **blob:** filesystem BlobStore backend (Milestone 2b, slice 1) ([#40](https://github.com/joshmcarthur/trove-project/issues/40)) ([fce5fcd](https://github.com/joshmcarthur/trove-project/commit/fce5fcd87c56304bbf1de6dacbb29d6669fa74d6))
* **config:** [[types]] declarations (type catalog 6/14) ([#73](https://github.com/joshmcarthur/trove-project/issues/73)) ([419291b](https://github.com/joshmcarthur/trove-project/commit/419291bbdc59ede4bab88231b6f8e69ee7ca9e34))
* **config:** implement TOML config loader ([410450a](https://github.com/joshmcarthur/trove-project/commit/410450a64797e8ac1b5b54db6df05e6eb38d5cdc))
* **gateway:** pluggable auth validators via http-gateway module ([5730a5c](https://github.com/joshmcarthur/trove-project/commit/5730a5c285e906e090ce0adb1912f08b30c3b72d))
* **journal:** implement Query with type, source, and time filters ([901de07](https://github.com/joshmcarthur/trove-project/commit/901de07cb607ea95692ea452bc71e65383dacf02))
* **journal:** optional retention_days pruning on startup ([5c5e760](https://github.com/joshmcarthur/trove-project/commit/5c5e760fee188f44e683d7ba9053ff4efd40e51e))
* **journal:** persist schema_ref on events ([7648b61](https://github.com/joshmcarthur/trove-project/commit/7648b6103f86ac3cc038ce31c0480fa64a1c292c))
* **mqtt-source:** enable reconnect and subscription healthcheck ([7ce0646](https://github.com/joshmcarthur/trove-project/commit/7ce06467741767f3b684fc8099deda85ba3b6381))
* **router:** guarantee dispatch via durable journal cursor ([f10d576](https://github.com/joshmcarthur/trove-project/commit/f10d576e9e7d1a72afda666d6b42d6e58f08fecc))
* **rpc:** add schema_ref to Event message ([0f0faf1](https://github.com/joshmcarthur/trove-project/commit/0f0faf198c12360b99c07d8ebdd92433bbd318a8))
* **runtime:** startup type catalog build (type catalog 10/14) ([#76](https://github.com/joshmcarthur/trove-project/issues/76)) ([9cf7fef](https://github.com/joshmcarthur/trove-project/commit/9cf7fef346d67c674ff2865c295117164b112dcc))
* **trove:** coordinate graceful shutdown on SIGTERM ([9e3032a](https://github.com/joshmcarthur/trove-project/commit/9e3032a45e474c1fb6c125d35cb8a131584bbf69))
* **types:** add trove:// type URI parse and format helpers ([19d717e](https://github.com/joshmcarthur/trove-project/commit/19d717ec112f111685ce9c25f1b306fbf9439fc1))
* **types:** blob-backed schema storage (type catalog 4/14) ([#71](https://github.com/joshmcarthur/trove-project/issues/71)) ([40e0b75](https://github.com/joshmcarthur/trove-project/commit/40e0b75549409558a452ffa170b443e796779ad3))
* **types:** catalog-backed emit validation (type catalog 9/14) ([#75](https://github.com/joshmcarthur/trove-project/issues/75)) ([0dd67a1](https://github.com/joshmcarthur/trove-project/commit/0dd67a16c1e5d744e81e45e95535ffe100804734))
* **types:** in-memory type catalog (type catalog 5/14) ([#72](https://github.com/joshmcarthur/trove-project/issues/72)) ([4cb024e](https://github.com/joshmcarthur/trove-project/commit/4cb024e77c65d60535594b42fb157fd071e2f688))
* **types:** JTD compile and payload validation (type catalog 3/14) ([#70](https://github.com/joshmcarthur/trove-project/issues/70)) ([8e07ba8](https://github.com/joshmcarthur/trove-project/commit/8e07ba8d9dc3c8821718b7c4362882385edcaa80))
* **types:** migrate modules to trove:// URIs (type catalog 11/14) ([#77](https://github.com/joshmcarthur/trove-project/issues/77)) ([ee0b859](https://github.com/joshmcarthur/trove-project/commit/ee0b8598f1e520f48eb08f9478b718aef2e359a7))
* **types:** parse and validate TTD envelopes ([8469f47](https://github.com/joshmcarthur/trove-project/commit/8469f4746056dafd14617900eb5abc227db1d582))

## Changelog

All notable changes to this project are documented in this file.

Release notes are generated by [release-please](https://github.com/googleapis/release-please)
from [Conventional Commits](https://www.conventionalcommits.org/) on `main`.
