// Package generation exposes immutable, runtime-safe facts about a sealed build.
package generation

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	embeddedBegin = []byte("VIVY_GENERATION_V1_BEGIN[")
	embeddedEnd   = []byte("]VIVY_GENERATION_V1_END")
)

// EmbeddedManifestBase64 is set only by vivy-sdk pack through the Go linker.
// A packed executable therefore carries the exact sealed Manifest it reports.
var EmbeddedManifestBase64 string

func EmbeddedManifest() ([]byte, error) {
	if EmbeddedManifestBase64 == "" {
		return nil, errors.New("no sealed Generation Manifest is embedded")
	}
	return decodeEmbedded([]byte(EmbeddedManifestBase64))
}

// FrameEmbeddedManifest makes a linker value self-delimiting so inspection
// can recover it from executable bytes without running untrusted code.
func FrameEmbeddedManifest(raw []byte) string {
	return string(embeddedBegin) + base64.StdEncoding.EncodeToString(raw) + string(embeddedEnd)
}

// ExtractEmbeddedManifest reads the framed linker value from executable
// bytes. The markers may also exist as standalone constants, so only a unique
// candidate whose body is valid base64 is accepted.
func ExtractEmbeddedManifest(binary []byte) ([]byte, error) {
	var found []byte
	for offset := 0; ; {
		start := bytes.Index(binary[offset:], embeddedBegin)
		if start < 0 {
			break
		}
		start += offset
		bodyStart := start + len(embeddedBegin)
		end := bytes.Index(binary[bodyStart:], embeddedEnd)
		if end >= 0 {
			candidate := binary[start : bodyStart+end+len(embeddedEnd)]
			decoded, err := decodeEmbedded(candidate)
			if err == nil && len(decoded) > 0 {
				if found != nil && !bytes.Equal(found, decoded) {
					return nil, errors.New("multiple sealed Generation Manifests are embedded")
				}
				if found == nil {
					found = decoded
				}
			}
		}
		offset = start + len(embeddedBegin)
	}
	if found == nil {
		return nil, errors.New("no sealed Generation Manifest is embedded")
	}
	return found, nil
}

func decodeEmbedded(framed []byte) ([]byte, error) {
	if !bytes.HasPrefix(framed, embeddedBegin) || !bytes.HasSuffix(framed, embeddedEnd) {
		return nil, errors.New("invalid sealed Generation Manifest framing")
	}
	body := framed[len(embeddedBegin) : len(framed)-len(embeddedEnd)]
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("decode sealed Generation Manifest: %w", err)
	}
	return decoded, nil
}

type CapabilityState string

const (
	NotCompiled  CapabilityState = "NOT_COMPILED"
	Unconfigured CapabilityState = "UNCONFIGURED"
)

type Manifest struct {
	Modules          []string
	Channels         []string
	Tools            []string
	ToolWorlds       []string
	ProviderProfiles []string
	ContextSources   []string
	SkillSources     []string
	Face             string
	NetworkStates    map[string]CapabilityState
}
