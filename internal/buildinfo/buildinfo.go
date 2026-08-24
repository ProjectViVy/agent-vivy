// Package buildinfo holds the binary identity stamped at link time.
package buildinfo

// Version is the species build id. Release builds override it with
// -ldflags "-X agent-vivy/internal/buildinfo.Version=...".
// identity 由 Vivy Studio 开发场地维护。
var Version = "dev"
