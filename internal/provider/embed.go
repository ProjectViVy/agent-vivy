package provider

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
)

// Provider data is embedded into the binary at build time (decision D3): the
// running process never opens a provider data file from disk, so there is no
// directory setting and no working-directory dependency. The data lives in a
// subdirectory of this package because //go:embed cannot reference a parent
// directory, which is also why data/ is the single write point (D1).
//
//go:embed data/*.yaml
var dataFS embed.FS

// embeddedDataDir is the only directory provider data may live in.
const embeddedDataDir = "data"

// EmbeddedDataFiles returns the embedded provider data documents in stable
// order. It is a diagnostics and review seam; nothing in the runtime needs to
// know the file names.
func EmbeddedDataFiles() []string {
	paths, err := fs.Glob(dataFS, embeddedDataDir+"/*.yaml")
	if err != nil {
		return nil
	}
	sort.Strings(paths)
	return paths
}

// LoadEmbedded parses and validates every embedded vendor document and
// returns the union. Validation is strict and collects every failure, so a
// broken data edit fails at startup with all of its problems listed rather
// than one at a time.
func LoadEmbedded() ([]Vendor, error) {
	paths := EmbeddedDataFiles()
	if len(paths) == 0 {
		return nil, errors.New("provider: the embedded provider data is empty")
	}
	vendors := make([]Vendor, 0, 64)
	var errs []error
	for _, name := range paths {
		raw, err := dataFS.ReadFile(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("provider: read embedded %s: %w", path.Base(name), err))
			continue
		}
		parsed, err := ParseVendors(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("provider: %s: %w", path.Base(name), err))
			continue
		}
		vendors = append(vendors, parsed...)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	// Re-validate the union so vendor-name and endpoint-identity uniqueness
	// also hold across documents, not just inside one.
	if err := validateVendors(vendors); err != nil {
		return nil, err
	}
	return vendors, nil
}
