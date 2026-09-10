package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agent-vivy/sdk/module"
	"gopkg.in/yaml.v3"
)

const descriptorFilename = "vivy-module.yaml"

// loadDescriptor reads one strict v1 Descriptor and returns its canonical
// semantic JSON bytes. It performs no source loading or runtime activation.
func loadDescriptor(dir string) (module.Descriptor, []byte, error) {
	filename := filepath.Join(dir, descriptorFilename)
	raw, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rejectLegacyDescriptor(dir)
		}
		return module.Descriptor{}, nil, fmt.Errorf("sdk: read %s: %w", descriptorFilename, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var descriptor module.Descriptor
	if err := decoder.Decode(&descriptor); err != nil {
		if strings.Contains(err.Error(), "already defined") {
			return module.Descriptor{}, nil, fmt.Errorf("sdk: parse %s: duplicate YAML key: %w", descriptorFilename, err)
		}
		return module.Descriptor{}, nil, fmt.Errorf("sdk: parse %s: %w", descriptorFilename, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are forbidden")
		}
		return module.Descriptor{}, nil, fmt.Errorf("sdk: parse %s: %w", descriptorFilename, err)
	}

	if descriptor.I18N != nil {
		normalized := descriptor.I18N.Normalize()
		descriptor.I18N = &normalized
	}
	if err := descriptor.Validate(); err != nil {
		return module.Descriptor{}, nil, fmt.Errorf("sdk: validate %s: %w", descriptorFilename, err)
	}
	canonical, err := json.Marshal(descriptor)
	if err != nil {
		return module.Descriptor{}, nil, fmt.Errorf("sdk: canonicalize %s: %w", descriptorFilename, err)
	}
	return descriptor, canonical, nil
}

func rejectLegacyDescriptor(dir string) (module.Descriptor, []byte, error) {
	legacyFilename := filepath.Join(dir, "vivy-plugin.json")
	raw, err := os.ReadFile(legacyFilename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return module.Descriptor{}, nil, fmt.Errorf("sdk: read %s: %w", descriptorFilename, os.ErrNotExist)
		}
		return module.Descriptor{}, nil, fmt.Errorf("sdk: read vivy-plugin.json: %w", err)
	}
	var header struct {
		APIVersion string `json:"apiVersion"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return module.Descriptor{}, nil, fmt.Errorf("sdk: legacy descriptor is not valid JSON: %w", err)
	}
	return module.Descriptor{}, nil, fmt.Errorf("unsupported apiVersion %s (want %s)", header.APIVersion, module.APIVersionV1)
}
