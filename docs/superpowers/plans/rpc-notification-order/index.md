# RPC notification-order implementation plan

> **For agentic workers:** Use superpowers:executing-plans for this coupled fix.

**Goal:** Preserve notification wire order without blocking bidirectional RPC.
**Architecture:** One Peer-owned bounded notification queue and worker;
responses stay on the reader, ID-bearing requests remain concurrent.
**Tech stack:** Go 1.26.8, existing JSONL/WebSocket transports, standard library.
**Spec:** `../../specs/2026-10-08-rpc-notification-order-design.md`

## Global constraints

- Preserve Journal, stream integrity checks, authentication, and writer behavior.
- No dependencies or Eino changes; see the spec's capability check.
- No silent event loss; queue overflow closes the Peer explicitly.
- Commits use the human owner's identity; no push or PR without authorization.

## Review focus

Callback reentrancy, saturated notification queue, callback cancellation,
pending calls during close, and cross-method ordering are covered by S01.

## Stories

- [S01: Ordered notification dispatch and regression proof](s01.md)
