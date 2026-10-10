# Laputa dependency closure

Pin the full reviewed Laputa revision `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24` together with garden/laputa/mentle pseudo-versions and their downloaded module checksums. Host and dependency Go declarations remain 1.26.4. Bootstrap rejects revision drift, wrong module identity, different Go declarations, and dirty sibling sources even at the expected HEAD.

Catalog metadata for optional cognitive, memory, and mask modules now lives in implementation-free catalog packages. Existing owners alias these identifiers; defaults imports the metadata, so compiling the SDK no longer compiles the cognitive implementation merely to enumerate it. Generated wiring still selects concrete owners only for recipes requiring them.

Scope is M1 of the authorized PR #46 repair. No new Ports, runtime dependency loader, or alternate assembly path. Source-bound conformance results and final clean packing validation are refreshed once after all M1–M6 internal changes are integrated.

## Module record

Existing build-owned T1 modules: vivy/diva-cognitive, vivy/diva-memory, vivy/memory-bml and its sync/tools contributions, vivy/masks. Source remains file:internal; cataloged core/cognitive-factory@v1, core/mask-service@v1, std/control-action@v1, std/context-source@v1, std/observer/run@v1 and std/tool@v1 consumers and lifecycle remain unchanged. No Grants or recipe selection change. Existing conformance/Inspect producers remain authoritative.

## Upstream review

Reviewed the required agentapi additions between ff3936f and 4b2bec2: WithMissionRevision provides a pinned effect fence; CaptureActivity is host-only and excluded from transport JSON; LookupCapture checks the bound host session and durable receipt; ArchiveCapturedSession uses the bound lifecycle owner. The new revision includes their integration and recovery tests and retains the pinned Go/Eino versions.
