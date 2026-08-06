// Package storage defines the Vivy-owned durable contracts — Journal,
// SnapshotStore, BlobStore, LeaseStore (D-026) — and the V0 SQLite
// reference backend built on modernc.org/sqlite (pure Go, no CGO).
//
// Domain code must never depend on SQLite-specific surfaces: SQL
// transactions as exposed types, partial indexes, FK cascades, AUTOINCREMENT,
// sqlc structs, FTS, or WAL-specific behavior (D-027). Those are adapter
// details behind the four contracts.
//
// The backend must pass the conformance suite (D-032, >=16 cases) before
// domain code trusts it. Versioned migrations apply on startup and a
// failure aborts startup (FR-8).
//
// Skeleton stage: contracts + SQLite backend implemented (B4); the
// conformance suite (D-032) lands in B5.
package storage
