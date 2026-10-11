package bundlecontract

import "strings"

// BindingRef is a secret-free pointer to an environment variable owned by the
// Vivy host. It is safe to retain in a run snapshot; the value is never read
// or copied into the snapshot.
type BindingRef struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
}

// ValidBindingReference accepts environment-variable names only. Bundle
// settings cannot persist literal values, interpolation expressions, or paths.
func ValidBindingReference(value string) bool {
	if value == "" || len(value) > 128 || !envNameStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if !envNameStart(value[i]) && (value[i] < '0' || value[i] > '9') {
			return false
		}
	}
	return true
}

// BindingName is the closed selector vocabulary for manifest-declared slots.
// envFrom slots are addressed by target environment name, never by array order.
func ValidBindingName(value string) bool {
	if value == "authEnv" {
		return true
	}
	if !strings.HasPrefix(value, "envFrom:") {
		return false
	}
	return ValidBindingReference(strings.TrimPrefix(value, "envFrom:"))
}

func envNameStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}
