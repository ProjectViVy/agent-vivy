// Package domain defines the Vivy-owned contract types that every other
// package speaks. It is the anti-leak firewall (D-007): domain imports
// nothing external — no Eino, no SQLite, no net/http. Engine and storage
// details stay behind their adapters.
package domain

// SessionID identifies a conversation session.
type SessionID string

// RunID identifies one agent run (one user message -> one run).
type RunID string

// EventSeq is a per-run monotonic event sequence number, starting at 1.
type EventSeq int64
