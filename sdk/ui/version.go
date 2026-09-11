// Package ui exposes the build-time identity of the public TypeScript UI SDK.
// The metadata is embedded from this package's package.json so Go Assembly
// generation and the Vite configuration are pinned to the same source file.
package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// packageJSON is deliberately embedded rather than discovered from the
// process working directory. Generation therefore has no filesystem, shell,
// or network dependency when checking the UI SDK pin.
//
//go:embed package.json
var packageJSON []byte

// PackageMetadata is the exact package identity used by the UI build.
type PackageMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PinnedPackage parses the embedded package metadata on each call. Parsing is
// cheap and keeps callers' validation tied to the authoritative embedded
// bytes, while still returning malformed metadata as a deterministic error.
func PinnedPackage() (PackageMetadata, error) {
	var metadata PackageMetadata
	if err := json.Unmarshal(packageJSON, &metadata); err != nil {
		return PackageMetadata{}, fmt.Errorf("parse UI SDK package metadata: %w", err)
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Version = strings.TrimSpace(metadata.Version)
	if metadata.Name == "" || metadata.Version == "" {
		return PackageMetadata{}, fmt.Errorf("UI SDK package metadata requires name and version")
	}
	return metadata, nil
}

// PackageName returns the pinned package name. Invalid embedded metadata is a
// package-build error; callers that need an actionable diagnostic should use
// PinnedPackage directly.
func PackageName() string {
	metadata, err := PinnedPackage()
	if err != nil {
		return ""
	}
	return metadata.Name
}

// Version returns the pinned package version. Invalid embedded metadata is a
// package-build error; callers that need an actionable diagnostic should use
// PinnedPackage directly.
func Version() string {
	metadata, err := PinnedPackage()
	if err != nil {
		return ""
	}
	return metadata.Version
}
