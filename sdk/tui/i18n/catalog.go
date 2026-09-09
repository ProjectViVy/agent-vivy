// Package i18n owns the immutable translation catalog used by the terminal
// faces. The Web face maintains a separate catalog with the same semantic key
// namespace.
package i18n

import (
	"fmt"
	"regexp"

	corei18n "agent-vivy/internal/i18n"
)

var namedPlaceholder = regexp.MustCompile(`\{\{([[:word:]]+)\}\}`)

// Translator resolves terminal presentation copy for one locale. It is a
// value so each view model can own its locale without shared mutable state.
type Translator struct {
	locale corei18n.Locale
}

// New returns a translator for locale. Locale validation remains owned by the
// core resolver; an unrecognized value safely falls back to English messages.
func New(locale corei18n.Locale) Translator {
	return Translator{locale: locale}
}

// Locale returns the locale selected for this translator.
func (t Translator) Locale() corei18n.Locale {
	return t.locale
}

// T resolves key in the selected locale, then English, then returns the key as
// a visible diagnostic. Named placeholders without a supplied argument remain
// unchanged so missing interpolation data is not silently hidden.
func (t Translator) T(key string, args map[string]any) string {
	message, ok := catalogFor(t.locale)[key]
	if !ok {
		message, ok = englishCatalog[key]
	}
	if !ok {
		message = key
	}
	if len(args) == 0 {
		return message
	}
	return namedPlaceholder.ReplaceAllStringFunc(message, func(placeholder string) string {
		match := namedPlaceholder.FindStringSubmatch(placeholder)
		if len(match) != 2 {
			return placeholder
		}
		value, exists := args[match[1]]
		if !exists {
			return placeholder
		}
		return fmt.Sprint(value)
	})
}

func catalogFor(locale corei18n.Locale) map[string]string {
	switch locale {
	case corei18n.English:
		return englishCatalog
	case corei18n.Chinese:
		return chineseCatalog
	default:
		return nil
	}
}
