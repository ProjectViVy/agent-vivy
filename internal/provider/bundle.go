package provider

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Backend identifiers allowed by schemas/providers.bundle.schema.json.
const (
	// BackendEinoOpenAI is the online eino-ext OpenAI component for
	// OpenAI-compatible endpoints (gateways, DeepSeek, ZAI, Kimi, custom).
	BackendEinoOpenAI = "eino-ext/openai"
	// BackendEinoClaude is the online eino-ext Claude component for the
	// Anthropic Messages API (research §8.5: the eino-ext component exists,
	// so the self-owned adapter milestone was dropped).
	BackendEinoClaude = "eino-ext/claude"
)

var (
	bundleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	envKeyPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// Bundle is the Vivy provider bundle document (D-022..D-025): shape and
// defaults only, adapted from the Diva providers.yaml vocabulary with a
// mandatory provenance record (D-025). Secrets never live here; env_key
// names the environment variable read at request time (D-010).
type Bundle struct {
	Name                  string          `yaml:"name"`
	APIType               string          `yaml:"api_type"`
	Keywords              []string        `yaml:"keywords"`
	EnvKey                string          `yaml:"env_key"`
	DisplayName           string          `yaml:"display_name"`
	DefaultModel          string          `yaml:"default_model"`
	GatewayPrefix         string          `yaml:"gateway_prefix"`
	SkipPrefixes          []string        `yaml:"skip_prefixes"`
	EnvExtras             []string        `yaml:"env_extras"`
	IsGateway             bool            `yaml:"is_gateway"`
	IsLocal               bool            `yaml:"is_local"`
	DetectByKeyPrefix     string          `yaml:"detect_by_key_prefix"`
	DetectByBaseKeyword   string          `yaml:"detect_by_base_keyword"`
	DefaultAPIBase        string          `yaml:"default_api_base"`
	StripModelPrefix      bool            `yaml:"strip_model_prefix"`
	SupportsPromptCaching bool            `yaml:"supports_prompt_caching"`
	Models                []string        `yaml:"models"`
	ModelOverrides        []ModelOverride `yaml:"model_overrides"`
	Backend               string          `yaml:"backend"`
	Provenance            Provenance      `yaml:"provenance"`
}

// ModelOverride records per-model deviations from bundle defaults.
type ModelOverride struct {
	Model                 string `yaml:"model"`
	APIBase               string `yaml:"api_base"`
	SupportsPromptCaching *bool  `yaml:"supports_prompt_caching"`
}

// Provenance cites the Diva catalog entry the bundle was re-derived from
// (D-025). Every bundle must carry it.
type Provenance struct {
	Source    string `yaml:"source"`
	Entry     string `yaml:"entry"`
	DerivedAt string `yaml:"derived_at"`
	Note      string `yaml:"note"`
}

// LoadBundle reads one bundle YAML from path and validates it against
// the Vivy bundle schema. Decoding is strict: unknown fields fail so a
// drifting schema surfaces at load time, not as a silent default.
func LoadBundle(path string) (Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Bundle{}, fmt.Errorf("provider: read bundle: %w", err)
	}
	return ParseBundle(data)
}

// ParseBundle decodes and validates raw bundle bytes (test seam).
func ParseBundle(data []byte) (Bundle, error) {
	var b Bundle
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&b); err != nil {
		return Bundle{}, fmt.Errorf("provider: parse bundle: %w", err)
	}
	if err := b.validate(); err != nil {
		return Bundle{}, err
	}
	return b, nil
}

func (b *Bundle) validate() error {
	var errs []error
	for _, f := range []struct{ field, value string }{
		{"name", b.Name},
		{"api_type", b.APIType},
		{"env_key", b.EnvKey},
		{"display_name", b.DisplayName},
		{"default_model", b.DefaultModel},
		{"default_api_base", b.DefaultAPIBase},
		{"backend", b.Backend},
	} {
		if strings.TrimSpace(f.value) == "" {
			errs = append(errs, fmt.Errorf("provider bundle: field %q is required", f.field))
		}
	}
	if b.Name != "" && !bundleNamePattern.MatchString(b.Name) {
		errs = append(errs, fmt.Errorf("provider bundle: name %q must match %s", b.Name, bundleNamePattern))
	}
	if b.EnvKey != "" && !envKeyPattern.MatchString(b.EnvKey) {
		errs = append(errs, fmt.Errorf("provider bundle: env_key %q must match %s", b.EnvKey, envKeyPattern))
	}
	switch b.APIType {
	case "openai", "anthropic":
	default:
		if b.APIType != "" {
			errs = append(errs, fmt.Errorf("provider bundle: api_type %q unsupported; want openai or anthropic", b.APIType))
		}
	}
	switch b.Backend {
	case BackendEinoOpenAI, BackendEinoClaude:
	default:
		if b.Backend != "" {
			errs = append(errs, fmt.Errorf("provider bundle: backend %q unsupported; want %s or %s", b.Backend, BackendEinoOpenAI, BackendEinoClaude))
		}
	}
	if len(b.Models) == 0 {
		errs = append(errs, errors.New("provider bundle: models must list at least one model"))
	} else if b.DefaultModel != "" {
		found := false
		for _, m := range b.Models {
			if m == b.DefaultModel {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("provider bundle: default_model %q must appear in models", b.DefaultModel))
		}
	}
	if b.Provenance.Source == "" || b.Provenance.Entry == "" || b.Provenance.DerivedAt == "" {
		errs = append(errs, errors.New("provider bundle: provenance must carry source, entry and derived_at (D-025)"))
	}
	return errors.Join(errs...)
}
