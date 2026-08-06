// Package domain defines the Vivy-owned contract types that every other
// package speaks: Session, Message, Run, RunEvent, ProviderRef, ToolSpec,
// ToolCall, Approval, MemoryItem, plus the Run state machine
// (accepted -> queued/active -> completed/failed/cancelled) and the
// ten-event minimum vocabulary (run.started ... run.cancelled).
//
// This package has ZERO external dependencies: no Eino types, no SQLite
// types, no HTTP types (PRD D-007). It is the anti-leak firewall.
//
// Skeleton stage: empty. Implemented in task B3.
package domain
