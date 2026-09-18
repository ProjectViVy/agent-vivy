package provider

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file is the single write point for provider configuration data
// (docs/plans/provider-registry/DESIGN.md §3). The document is embedded at
// build time (embed.go) and validated strictly here: an unknown key is a
// hard error, and every failure is collected rather than returned one at a
// time. Data supplies data and never capability — a vendor entry may name
// one of the sealed adapters, an address, models and metadata, and nothing
// executable.

// Vendor ids and credential names may lead with a digit: upstream catalogs
// carry them (e.g. the vendor "302ai" and its "302AI_API_KEY"), and both are
// usable as map keys and process-environment names. envKeyPattern must stay
// textually identical to config.ValidEnvKey's rule, which the credential
// allowlist applies to a Profile's SecretRefs (PROV-P1 data rule 2).
var (
	vendorNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	envKeyPattern     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_]*$`)
)

// Model is one model id offered by an endpoint, with the metadata Vivy
// asserts about it. A zero ContextWindow means unknown, and every consumer
// keeps its conservative default (DESIGN.md §6); an unpriced model is
// unpriced, never free.
type Model struct {
	ID               string  `yaml:"id"`
	ContextWindow    int     `yaml:"context_window"`
	InputPerMTok     float64 `yaml:"input_per_mtok"`
	OutputPerMTok    float64 `yaml:"output_per_mtok"`
	SupportsImages   bool    `yaml:"supports_images"`
	SupportsThinking bool    `yaml:"supports_thinking"`
}

// Endpoint is one wire protocol at one address for one vendor. Its identity
// is (Adapter, BaseURL) (DESIGN.md §4.1), which is why a vendor speaking two
// protocols is ordinary data rather than a special case.
type Endpoint struct {
	Adapter               string   `yaml:"adapter"`
	BaseURL               string   `yaml:"base_url"`
	DefaultModel          string   `yaml:"default_model"`
	Models                []Model  `yaml:"models"`
	Capabilities          []string `yaml:"capabilities"`
	SupportsPromptCaching bool     `yaml:"supports_prompt_caching"`
}

// Model returns the metadata for id, if the endpoint declares it.
func (e Endpoint) Model(id string) (Model, bool) {
	for _, model := range e.Models {
		if model.ID == id {
			return model, true
		}
	}
	return Model{}, false
}

// ModelIDs returns the endpoint's model ids in declared order.
func (e Endpoint) ModelIDs() []string {
	ids := make([]string, 0, len(e.Models))
	for _, model := range e.Models {
		ids = append(ids, model.ID)
	}
	return ids
}

// HasCapability reports whether the endpoint declares capability.
func (e Endpoint) HasCapability(capability string) bool {
	for _, declared := range e.Capabilities {
		if declared == capability {
			return true
		}
	}
	return false
}

// Vendor is one provider identity: whose endpoint it is and which
// environment variable carries its credential. Secrets never live here
// (D-010) — EnvKey is the name of the variable, never its value.
type Vendor struct {
	Name        string     `yaml:"name"`
	DisplayName string     `yaml:"display_name"`
	EnvKey      string     `yaml:"env_key"`
	Endpoints   []Endpoint `yaml:"endpoints"`
	Provenance  Provenance `yaml:"provenance"`
}

// DefaultEndpoint returns the vendor's first declared endpoint, which is the
// vendor's default protocol (DESIGN.md §5: an empty settings base_url means
// this endpoint).
func (v Vendor) DefaultEndpoint() (Endpoint, bool) {
	if len(v.Endpoints) == 0 {
		return Endpoint{}, false
	}
	return v.Endpoints[0], true
}

// Endpoint returns the endpoint addressed by (adapter, baseURL).
func (v Vendor) Endpoint(adapter, baseURL string) (Endpoint, bool) {
	for _, endpoint := range v.Endpoints {
		if endpoint.Adapter == adapter && endpoint.BaseURL == baseURL {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

// Provenance cites the Diva catalog entry a vendor was re-derived from
// (D-025). Every vendor must carry it.
type Provenance struct {
	Source    string `yaml:"source"`
	Entry     string `yaml:"entry"`
	DerivedAt string `yaml:"derived_at"`
	Note      string `yaml:"note"`
}

// ParseVendors decodes and validates one embedded vendor document. Decoding
// is strict: an unknown key fails, so a drifting schema surfaces at load
// time rather than as a silent default.
func ParseVendors(data []byte) ([]Vendor, error) {
	var vendors []Vendor
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&vendors); err != nil {
		return nil, fmt.Errorf("provider: parse vendor data: %w", err)
	}
	if len(vendors) == 0 {
		return nil, errors.New("provider: vendor data declares no vendors")
	}
	if err := validateVendors(vendors); err != nil {
		return nil, err
	}
	return vendors, nil
}

// validateVendors applies every load-time rule in DESIGN.md §3.4 and joins
// all failures.
func validateVendors(vendors []Vendor) error {
	var errs []error
	seenNames := make(map[string]struct{}, len(vendors))
	seenEndpoints := make(map[string]string)
	for i, vendor := range vendors {
		where := fmt.Sprintf("provider data: vendor[%d]", i)
		if vendor.Name != "" {
			where = fmt.Sprintf("provider data: vendor %q", vendor.Name)
		}
		errs = append(errs, validateVendor(vendor, where, seenNames, seenEndpoints)...)
	}
	return errors.Join(errs...)
}

func validateVendor(vendor Vendor, where string, seenNames map[string]struct{}, seenEndpoints map[string]string) []error {
	var errs []error
	if strings.TrimSpace(vendor.Name) == "" {
		errs = append(errs, fmt.Errorf("%s: field \"name\" is required", where))
	} else if !vendorNamePattern.MatchString(vendor.Name) {
		errs = append(errs, fmt.Errorf("%s: name must match %s", where, vendorNamePattern))
	} else if _, duplicate := seenNames[vendor.Name]; duplicate {
		errs = append(errs, fmt.Errorf("%s: duplicate vendor name", where))
	} else {
		seenNames[vendor.Name] = struct{}{}
	}
	if strings.TrimSpace(vendor.EnvKey) == "" {
		errs = append(errs, fmt.Errorf("%s: field \"env_key\" is required", where))
	} else if !envKeyPattern.MatchString(vendor.EnvKey) {
		errs = append(errs, fmt.Errorf("%s: env_key %q must match %s", where, vendor.EnvKey, envKeyPattern))
	}
	if strings.TrimSpace(vendor.DisplayName) == "" {
		errs = append(errs, fmt.Errorf("%s: field \"display_name\" is required", where))
	}
	if vendor.Provenance.Source == "" || vendor.Provenance.Entry == "" || vendor.Provenance.DerivedAt == "" {
		errs = append(errs, fmt.Errorf("%s: provenance must carry source, entry and derived_at (D-025)", where))
	}
	if len(vendor.Endpoints) == 0 {
		errs = append(errs, fmt.Errorf("%s: at least one endpoint is required", where))
	}
	for j, endpoint := range vendor.Endpoints {
		errs = append(errs, validateEndpoint(vendor, endpoint, fmt.Sprintf("%s: endpoint[%d]", where, j), seenEndpoints)...)
	}
	return errs
}

func validateEndpoint(vendor Vendor, endpoint Endpoint, where string, seenEndpoints map[string]string) []error {
	var errs []error
	adapterLabel := endpoint.Adapter
	if adapterLabel == "" {
		adapterLabel = "(missing)"
	}
	if endpoint.BaseURL != "" {
		where = fmt.Sprintf("%s (%s %s)", where, adapterLabel, endpoint.BaseURL)
	}
	if endpoint.Adapter == "" {
		errs = append(errs, fmt.Errorf("%s: field \"adapter\" is required", where))
	} else if !IsSealedAdapter(endpoint.Adapter) {
		errs = append(errs, fmt.Errorf("%s: adapter %q is not sealed; want one of %s", where, endpoint.Adapter, strings.Join(AdapterFamilies(), ", ")))
	}
	if strings.TrimSpace(endpoint.BaseURL) == "" {
		errs = append(errs, fmt.Errorf("%s: field \"base_url\" is required", where))
	} else if parsed, err := url.Parse(endpoint.BaseURL); err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		errs = append(errs, fmt.Errorf("%s: base_url %q must be an absolute http or https URL", where, endpoint.BaseURL))
	}
	if strings.TrimSpace(endpoint.DefaultModel) == "" {
		errs = append(errs, fmt.Errorf("%s: field \"default_model\" is required", where))
	}
	if len(endpoint.Models) == 0 {
		errs = append(errs, fmt.Errorf("%s: models must list at least one model", where))
	}
	seenModels := make(map[string]struct{}, len(endpoint.Models))
	for k, model := range endpoint.Models {
		modelWhere := fmt.Sprintf("%s: model[%d]", where, k)
		if model.ID == "" || strings.TrimSpace(model.ID) != model.ID || strings.ContainsAny(model.ID, "\x00\r\n") {
			errs = append(errs, fmt.Errorf("%s: model id %q is invalid", modelWhere, model.ID))
			continue
		}
		modelWhere = fmt.Sprintf("%s: model %q", where, model.ID)
		if _, duplicate := seenModels[model.ID]; duplicate {
			errs = append(errs, fmt.Errorf("%s: duplicate model id", modelWhere))
		}
		seenModels[model.ID] = struct{}{}
		for _, prefix := range []string{vendor.Name + "/", endpoint.Adapter + "/"} {
			if prefix != "/" && strings.HasPrefix(model.ID, prefix) {
				errs = append(errs, fmt.Errorf("%s: model id carries a forbidden %q prefix; send the endpoint's raw model id", modelWhere, prefix))
				break
			}
		}
		if model.ContextWindow < 0 || model.InputPerMTok < 0 || model.OutputPerMTok < 0 {
			errs = append(errs, fmt.Errorf("%s: metadata must not be negative", modelWhere))
		}
	}
	if endpoint.DefaultModel != "" && len(endpoint.Models) > 0 {
		if _, ok := endpoint.Model(endpoint.DefaultModel); !ok {
			errs = append(errs, fmt.Errorf("%s: default_model %q must appear in models", where, endpoint.DefaultModel))
		}
	}
	for _, capability := range endpoint.Capabilities {
		if !AdapterSupportsCapability(endpoint.Adapter, capability) {
			errs = append(errs, fmt.Errorf("%s: capability %q is not implemented by adapter %q", where, capability, endpoint.Adapter))
		}
	}
	if endpoint.Adapter != "" && endpoint.BaseURL != "" {
		identity := endpoint.Adapter + "\x00" + endpoint.BaseURL
		if previous, duplicate := seenEndpoints[identity]; duplicate {
			errs = append(errs, fmt.Errorf("%s: endpoint (%s, %s) is already declared by vendor %q", where, endpoint.Adapter, endpoint.BaseURL, previous))
		} else {
			seenEndpoints[identity] = vendor.Name
		}
	}
	return errs
}
