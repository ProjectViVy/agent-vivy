package i18n

import "fmt"

// Locale identifies one supported presentation locale.
type Locale string

const (
	English Locale = "en"
	Chinese Locale = "zh"

	DefaultLocaleEnv = "VIVY_DEFAULT_LOCALE"
)

// Parse validates a locale identifier.
func Parse(value string) (Locale, error) {
	locale := Locale(value)
	switch locale {
	case English, Chinese:
		return locale, nil
	default:
		return "", fmt.Errorf("i18n: unsupported locale %q (want en or zh)", value)
	}
}

// Resolve returns the effective locale. A workspace override always wins. A
// sealed generation uses only its embedded locale, while an unsealed
// development body may use the developer default before its compiled value.
func Resolve(workspace string, generation Locale, developer Locale, sealed bool) (Locale, error) {
	if workspace != "" {
		return Parse(workspace)
	}
	if !sealed && developer != "" {
		return Parse(string(developer))
	}
	if generation != "" {
		return Parse(string(generation))
	}
	return English, nil
}
