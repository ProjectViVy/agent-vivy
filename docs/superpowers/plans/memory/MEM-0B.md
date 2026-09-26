# MEM-0B memory capability profile contract Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkbox syntax.

**Goal:** Freeze the host-facing memory capability profile and trusted-scope identity mapping that every provider adapter must implement or refuse explicitly.
**Architecture:** One architecture doc extending the SCX integration rows; terms reuse verified SDK types (`contextsource`, `observer`, `controlaction`, `status`).
**Tech Stack:** Markdown; SDK contract citations.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) REQ-MEM-1/6/7/8. Predecessor: MEM-0A pins upstream facts this doc references. State: [index](index.md).

## Global Constraints

- Cite real SDK symbols (`sdk/port/contextsource` etc.), not invented APIs.
- The profile is a conformance contract, not a Go interface — provider adapters
  keep provider-native behavior behind it.
- Unsupported operations must produce explicit errors; silent emulation banned.
- Plugins never derive authorization from query text or user-supplied metadata.

## Task 1: Envelope and capability declaration

**Files:**
- Create: `docs/architecture/VIVY-MEMORY-PROFILE.md`

- [ ] **Step 1:** Draft sections:

  1. **Record envelope**: provider ID, record ID, revision, source/evidence
     references, capture/effective time, expiry, namespaced provider metadata.
     Map to `contextsource.Candidate` fields (`SourceID/ContentID/Version/
     UpdatedAt/ValidUntil/Metadata`) and state what travels only in
     `controlaction` payloads.
  2. **Operation states**: accepted/pending/completed/failed + stable
     operation IDs; reuse `observer.DeliveryState` vocabulary.
  3. **Capability declaration**: the closed set {recall, evidence-read,
     ingestion, correction, deletion, export, exact-version, async-extraction,
     graph, skill} with per-provider truthful advertisement rules.
  4. **Error contract**: explicit `unsupported` errors; no silent emulation of
     deletion/version guarantees.
  5. **Score isolation**: provider scores never compared cross-backend;
     ContextHost retains final bounded selection.

- [ ] **Step 2:** Run review pass: every claim has a §"seam" pointer to the
  SDK symbol or host package that enforces it.

## Task 2: Trusted scope and identity mapping

- [ ] **Step 1:** In the same doc, define the scope model: extend
  `contextsource.Scope{TenantID,WorkspaceID,SessionID}` semantics with the two
  missing dimensions the issue requires — trusted **user** and
  **agent/persona** identity. Specify who assigns them (Host, from authenticated
  session/tenant context — never from request payload fields), remote
  account/namespace mapping for remote providers, and the failure mode when a
  mapping is absent (scope denied, not scope-widened).

- [ ] **Step 2:** Define selection/ownership rules from the issue verbatim
  where still true: one primary write destination per scope; mounted read
  sources; no implicit migration on switch; no auto-activation without
  credentials/network.

## Task 3: Commit

- [ ] **Step 1:** `git commit -m "docs(memory): G0 capability profile and scope contract"` after review; confirm `just fmt-check` unaffected (docs only).
