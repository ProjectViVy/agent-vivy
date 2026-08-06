// Package httpapi is the UI-facing command/query/event API. It exposes
// REST endpoints for sessions, messages, runs, and approvals, plus an SSE
// endpoint streaming persisted RunEvents.
//
// Hard boundary: handlers speak ONLY Vivy domain DTOs serialized as JSON
// (schemas/). No Eino types, no Go-internal types, no reference-project
// types cross this boundary (PRD D-007). Approval authority is enforced
// server-side here; UI-only checks are never authoritative (D-009).
//
// Skeleton stage: empty. Implemented in tasks D1/D2.
package httpapi
