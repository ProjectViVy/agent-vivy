// Package providerprofile defines the public, declarative
// std/provider-profile@v1 contract. It deliberately contains no executable
// model, constructor, callback, transport, or credential value.
package providerprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type EndpointClass string

const (
	EndpointNative  EndpointClass = "native"
	EndpointGateway EndpointClass = "gateway"
	EndpointLocal   EndpointClass = "local"
)

var (
	profileIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?:[./][a-z0-9][a-z0-9_-]*)*$`)
	secretRefPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// Profile is pure configuration metadata. SecretRefs contains names only;
// Secret values are resolved by the internal ModelHost at call time.
type Profile struct {
	ID            string
	AdapterFamily string
	ModelIDs      []string
	EndpointClass EndpointClass
	SecretRefs    []string
	OptionsSchema json.RawMessage
}

// Provider contributes exactly one declarative Profile to ModelHost.
type Provider interface {
	Definition() Profile
}

func (profile Profile) Clone() Profile {
	profile.ModelIDs = append([]string(nil), profile.ModelIDs...)
	profile.SecretRefs = append([]string(nil), profile.SecretRefs...)
	profile.OptionsSchema = append(json.RawMessage(nil), profile.OptionsSchema...)
	return profile
}

func (profile Profile) Validate() error {
	var failures []error
	if !profileIDPattern.MatchString(profile.ID) {
		failures = append(failures, fmt.Errorf("provider Profile id %q is invalid", profile.ID))
	}
	if strings.TrimSpace(profile.AdapterFamily) == "" || strings.TrimSpace(profile.AdapterFamily) != profile.AdapterFamily {
		failures = append(failures, errors.New("provider Profile adapter family is required and must be normalized"))
	}
	switch profile.EndpointClass {
	case EndpointNative, EndpointGateway, EndpointLocal:
	default:
		failures = append(failures, fmt.Errorf("provider Profile endpoint class %q is unsupported", profile.EndpointClass))
	}
	if len(profile.ModelIDs) == 0 {
		failures = append(failures, errors.New("provider Profile must declare at least one raw model id"))
	}
	seenModels := make(map[string]struct{}, len(profile.ModelIDs))
	nativePrefixes := []string{profile.ID + "/", strings.TrimSuffix(profile.AdapterFamily, "-compatible") + "/"}
	for _, modelID := range profile.ModelIDs {
		if modelID == "" || strings.TrimSpace(modelID) != modelID || strings.ContainsAny(modelID, "\x00\r\n") {
			failures = append(failures, fmt.Errorf("provider Profile raw model id %q is invalid", modelID))
			continue
		}
		if _, duplicate := seenModels[modelID]; duplicate {
			failures = append(failures, fmt.Errorf("provider Profile contains duplicate raw model id %q", modelID))
		}
		seenModels[modelID] = struct{}{}
		if profile.EndpointClass == EndpointNative {
			for _, prefix := range nativePrefixes {
				if prefix != "/" && strings.HasPrefix(modelID, prefix) {
					failures = append(failures, fmt.Errorf("provider Profile raw model id %q contains forbidden native provider prefix", modelID))
					break
				}
			}
		}
	}
	seenSecrets := make(map[string]struct{}, len(profile.SecretRefs))
	for _, reference := range profile.SecretRefs {
		if !secretRefPattern.MatchString(reference) {
			failures = append(failures, fmt.Errorf("provider Profile Secret reference %q must be an environment-variable name", reference))
			continue
		}
		if _, duplicate := seenSecrets[reference]; duplicate {
			failures = append(failures, fmt.Errorf("provider Profile contains duplicate Secret reference %q", reference))
		}
		seenSecrets[reference] = struct{}{}
	}
	if err := validateOptionsSchema(profile.OptionsSchema); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func validateOptionsSchema(raw json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if len(raw) == 0 || decoder.Decode(&object) != nil || object == nil {
		return errors.New("provider Profile option schema must be one JSON object")
	}
	var extra any
	if decoder.Decode(&extra) == nil {
		return errors.New("provider Profile option schema must contain one JSON value")
	}
	if schemaType, ok := object["type"]; !ok || schemaType != "object" {
		return errors.New("provider Profile option schema must declare type object")
	}
	return nil
}
