// Package events fans out persisted RunEvents to live subscribers (SSE)
// and serves the replay cursor: clients request events with after_seq=N
// and receive exactly the events that follow, with no gaps or duplicates
// across reconnects (FR-5, AS-7).
//
// The event source never depends on Eino's internal types; it reads from
// the persisted Journal, which is the single source of truth for history
// (D-017 logs-first).
//
// Skeleton stage: empty. Implemented in task D1.
package events
