// Package config loads and validates the typed configuration store
// (config.yaml) at startup. It is the boundary for the secret rule:
// non-secret values (model id, base URL, tool enablement) are queryable;
// secrets (provider keys) are never read into config and never persisted
// (PRD FR-10, D-010). Invalid config aborts startup.
//
// Skeleton stage: empty. Wired in task B2.
package config
