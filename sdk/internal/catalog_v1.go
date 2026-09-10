package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"agent-vivy/sdk/module"
)

const (
	catalogAPIVersion      = "vivy.i18n/v1"
	maxCatalogBytes        = 1 << 20
	maxCatalogUnits        = 4096
	maxCatalogMessageBytes = 8 << 10
	maxCatalogPlaceholders = 32
)

type catalogState string

const (
	catalogNotApplicable    catalogState = "NOT_APPLICABLE"
	catalogComplete         catalogState = "COMPLETE"
	catalogIncompleteLocale catalogState = "INCOMPLETE_LOCALE"
)

type localeState string

const (
	localeComplete   localeState = "COMPLETE"
	localeIncomplete localeState = "INCOMPLETE"
)

type compiledCatalog struct {
	SchemaVersion string
	Path          string
	Digest        string
	DefaultLocale string
	Locales       []string
	Completeness  map[string]localeState
	State         catalogState
	Canonical     []byte
}

type catalogDocument struct {
	APIVersion string                 `json:"apiVersion"`
	Units      map[string]catalogUnit `json:"units"`
}

type catalogUnit struct {
	Description  string            `json:"description"`
	Placeholders []string          `json:"placeholders"`
	Messages     map[string]string `json:"messages"`
	Short        map[string]string `json:"short,omitempty"`
	Long         map[string]string `json:"long,omitempty"`
}

var placeholderNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var placeholderTokenPattern = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

func loadCatalog(moduleRoot string, descriptor module.Descriptor) (compiledCatalog, error) {
	if descriptor.I18N == nil {
		return compiledCatalog{State: catalogNotApplicable}, nil
	}
	if err := descriptor.I18N.Validate(); err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: validate i18n descriptor for %s: %w", descriptor.Module.ID, err)
	}

	root, candidate, err := confinedCatalogPath(moduleRoot, descriptor.I18N.Catalog)
	if err != nil {
		return compiledCatalog{}, err
	}
	_ = root
	raw, err := os.ReadFile(candidate)
	if err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: read i18n catalog for %s: %w", descriptor.Module.ID, err)
	}
	if len(raw) > maxCatalogBytes {
		return compiledCatalog{}, fmt.Errorf("sdk: i18n catalog for %s exceeds %d bytes", descriptor.Module.ID, maxCatalogBytes)
	}
	if !utf8.Valid(raw) {
		return compiledCatalog{}, fmt.Errorf("sdk: i18n catalog for %s is not valid UTF-8", descriptor.Module.ID)
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: i18n catalog for %s: %w", descriptor.Module.ID, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document catalogDocument
	if err := decoder.Decode(&document); err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: parse i18n catalog for %s: %w", descriptor.Module.ID, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: parse i18n catalog for %s: %w", descriptor.Module.ID, err)
	}
	if document.APIVersion != catalogAPIVersion {
		return compiledCatalog{}, fmt.Errorf("sdk: unsupported i18n apiVersion %s for %s", document.APIVersion, descriptor.Module.ID)
	}
	if document.Units == nil {
		return compiledCatalog{}, fmt.Errorf("sdk: i18n catalog for %s requires units", descriptor.Module.ID)
	}
	if len(document.Units) > maxCatalogUnits {
		return compiledCatalog{}, fmt.Errorf("sdk: i18n catalog for %s exceeds %d units", descriptor.Module.ID, maxCatalogUnits)
	}

	locales := descriptor.I18N.Normalize().Locales
	allowedLocales := make(map[string]struct{}, len(locales))
	for _, locale := range locales {
		allowedLocales[locale] = struct{}{}
	}
	completeness := make(map[string]localeState, len(locales)+1)
	for _, locale := range locales {
		completeness[locale] = localeComplete
	}
	if _, exists := completeness["zh"]; !exists {
		completeness["zh"] = localeIncomplete
	}

	ownerPrefix := "plugin." + descriptor.Module.ID + "."
	for key, unit := range document.Units {
		if !strings.HasPrefix(key, ownerPrefix) || len(key) == len(ownerPrefix) {
			return compiledCatalog{}, fmt.Errorf("sdk: i18n key %s is outside owner namespace %s*", key, ownerPrefix)
		}
		if err := validateCatalogUnit(key, &unit, allowedLocales, completeness); err != nil {
			return compiledCatalog{}, err
		}
		document.Units[key] = unit
	}

	state := catalogComplete
	for _, locale := range completeness {
		if locale != localeComplete {
			state = catalogIncompleteLocale
			break
		}
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return compiledCatalog{}, fmt.Errorf("sdk: canonicalize i18n catalog for %s: %w", descriptor.Module.ID, err)
	}
	digestBytes := sha256.Sum256(canonical)
	return compiledCatalog{
		SchemaVersion: document.APIVersion,
		Path:          descriptor.I18N.Catalog,
		Digest:        hex.EncodeToString(digestBytes[:]),
		DefaultLocale: "en",
		Locales:       append([]string(nil), locales...),
		Completeness:  completeness,
		State:         state,
		Canonical:     canonical,
	}, nil
}

func confinedCatalogPath(moduleRoot, relative string) (string, string, error) {
	root, err := filepath.EvalSymlinks(moduleRoot)
	if err != nil {
		return "", "", fmt.Errorf("sdk: resolve Module source %s: %w", moduleRoot, err)
	}
	candidate, err := filepath.EvalSymlinks(filepath.Join(root, relative))
	if err != nil {
		return "", "", fmt.Errorf("sdk: resolve i18n catalog %s: %w", relative, err)
	}
	relativeToRoot, err := filepath.Rel(root, candidate)
	if err != nil || !filepath.IsLocal(relativeToRoot) {
		return "", "", fmt.Errorf("sdk: i18n catalog %s escapes Module source", relative)
	}
	return root, candidate, nil
}

func validateCatalogUnit(key string, unit *catalogUnit, allowedLocales map[string]struct{}, completeness map[string]localeState) error {
	if strings.TrimSpace(unit.Description) == "" {
		return fmt.Errorf("sdk: i18n unit %s requires a non-empty description", key)
	}
	if len(unit.Placeholders) > maxCatalogPlaceholders {
		return fmt.Errorf("sdk: i18n unit %s exceeds %d placeholders", key, maxCatalogPlaceholders)
	}
	if !sort.StringsAreSorted(unit.Placeholders) {
		return fmt.Errorf("sdk: i18n unit %s placeholders must be sorted", key)
	}
	declared := make(map[string]struct{}, len(unit.Placeholders))
	for _, name := range unit.Placeholders {
		if !placeholderNamePattern.MatchString(name) {
			return fmt.Errorf("sdk: i18n unit %s has invalid placeholder %q", key, name)
		}
		if _, exists := declared[name]; exists {
			return fmt.Errorf("sdk: i18n unit %s has duplicate placeholder %s", key, name)
		}
		declared[name] = struct{}{}
	}
	if len(unit.Messages) == 0 || strings.TrimSpace(unit.Messages["en"]) == "" {
		return fmt.Errorf("sdk: i18n unit %s is missing non-empty English message", key)
	}

	forms := []struct {
		name     string
		messages *map[string]string
		required bool
	}{
		{name: "messages", messages: &unit.Messages, required: true},
		{name: "short", messages: &unit.Short},
		{name: "long", messages: &unit.Long},
	}
	for _, form := range forms {
		if *form.messages == nil {
			continue
		}
		normalized, err := normalizeCatalogMessages(key, form.name, *form.messages, allowedLocales, declared)
		if err != nil {
			return err
		}
		if !form.required && strings.TrimSpace(normalized["en"]) == "" {
			return fmt.Errorf("sdk: i18n unit %s form %s is missing non-empty English message", key, form.name)
		}
		*form.messages = normalized
		for locale := range completeness {
			if strings.TrimSpace(normalized[locale]) == "" {
				completeness[locale] = localeIncomplete
			}
		}
	}
	return nil
}

func normalizeCatalogMessages(key, form string, messages map[string]string, allowedLocales, declared map[string]struct{}) (map[string]string, error) {
	normalized := make(map[string]string, len(messages))
	for rawLocale, message := range messages {
		locale := module.NormalizeLocale(rawLocale)
		if _, allowed := allowedLocales[locale]; !allowed {
			return nil, fmt.Errorf("sdk: i18n unit %s form %s uses unpackaged locale %s", key, form, locale)
		}
		if _, exists := normalized[locale]; exists {
			return nil, fmt.Errorf("sdk: i18n unit %s form %s has duplicate normalized locale %s", key, form, locale)
		}
		if len([]byte(message)) > maxCatalogMessageBytes {
			return nil, fmt.Errorf("sdk: i18n unit %s form %s locale %s exceeds %d bytes", key, form, locale, maxCatalogMessageBytes)
		}
		actual, err := messagePlaceholders(message)
		if err != nil {
			return nil, fmt.Errorf("sdk: i18n unit %s form %s locale %s: %w", key, form, locale, err)
		}
		if !reflect.DeepEqual(actual, declared) {
			return nil, fmt.Errorf("sdk: i18n unit %s form %s locale %s violates placeholder parity", key, form, locale)
		}
		normalized[locale] = message
	}
	return normalized, nil
}

func messagePlaceholders(message string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	consumed := placeholderTokenPattern.ReplaceAllStringFunc(message, func(token string) string {
		match := placeholderTokenPattern.FindStringSubmatch(token)
		name := match[1]
		if placeholderNamePattern.MatchString(name) {
			found[name] = struct{}{}
		}
		return ""
	})
	if strings.Contains(consumed, "{{") || strings.Contains(consumed, "}}") {
		return nil, fmt.Errorf("contains malformed placeholder")
	}
	for _, match := range placeholderTokenPattern.FindAllStringSubmatch(message, -1) {
		if !placeholderNamePattern.MatchString(match[1]) {
			return nil, fmt.Errorf("contains invalid placeholder %q", match[1])
		}
	}
	return found, nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %s", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	}
	if err := walk(); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are forbidden")
		}
		return err
	}
	return nil
}
