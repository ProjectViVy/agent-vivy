// Package abi is the leaf owner of the DIVA C-ABI contract version so
// tooling can validate the published header without compiling the embedded
// host (and its internal/app dependency chain).
package abi

// Version is the single source of truth for the DIVA C-ABI contract
// version. cmd/vivy-shared/vivy_abi.h must define VIVY_ABI_VERSION to the
// same value (checked by exports_test).
const Version = 1
