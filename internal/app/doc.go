// Package app composes all Vivy services and owns the process lifecycle:
// build config -> storage -> provider -> runtime -> events -> httpapi,
// start them in dependency order, and shut them down in reverse order
// with bounded grace on Windows.
//
// Skeleton stage: empty. Wired in tasks B2/C3/D1 (docs/TODO.md).
package app
