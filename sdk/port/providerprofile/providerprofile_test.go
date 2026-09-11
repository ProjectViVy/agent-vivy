package providerprofile

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProfileContainsNoExecutableProvider(t *testing.T) {
	typeOfProfile := reflect.TypeOf(Profile{})
	allowed := map[string]reflect.Kind{
		"ID": reflect.String, "AdapterFamily": reflect.String,
		"ModelIDs": reflect.Slice, "EndpointClass": reflect.String,
		"SecretRefs": reflect.Slice, "OptionsSchema": reflect.Slice,
	}
	if typeOfProfile.NumField() != len(allowed) {
		t.Fatalf("Profile fields = %d, want exactly %d declarative fields", typeOfProfile.NumField(), len(allowed))
	}
	for index := 0; index < typeOfProfile.NumField(); index++ {
		field := typeOfProfile.Field(index)
		kind, ok := allowed[field.Name]
		if !ok || field.Type.Kind() != kind {
			t.Fatalf("Profile field %s has executable or unknown type %s", field.Name, field.Type)
		}
	}
}

func TestProfileValidateRejectsInvalidDeclarativeData(t *testing.T) {
	valid := Profile{
		ID: "openai", AdapterFamily: "openai-compatible",
		ModelIDs: []string{"gpt-4o"}, EndpointClass: EndpointNative,
		SecretRefs:    []string{"OPENAI_API_KEY"},
		OptionsSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
	}
	tests := []struct {
		name   string
		mutate func(*Profile)
		want   string
	}{
		{name: "empty id", mutate: func(profile *Profile) { profile.ID = "" }, want: "id"},
		{name: "empty family", mutate: func(profile *Profile) { profile.AdapterFamily = "" }, want: "adapter family"},
		{name: "no models", mutate: func(profile *Profile) { profile.ModelIDs = nil }, want: "model"},
		{name: "duplicate model", mutate: func(profile *Profile) { profile.ModelIDs = []string{"gpt-4o", "gpt-4o"} }, want: "duplicate"},
		{name: "prefixed native model", mutate: func(profile *Profile) { profile.ModelIDs = []string{"openai/gpt-4o"} }, want: "raw model id"},
		{name: "inline secret", mutate: func(profile *Profile) { profile.SecretRefs = []string{"sk-live-secret"} }, want: "Secret reference"},
		{name: "invalid option schema", mutate: func(profile *Profile) { profile.OptionsSchema = json.RawMessage(`[]`) }, want: "option schema"},
		{name: "unknown endpoint class", mutate: func(profile *Profile) { profile.EndpointClass = "magic" }, want: "endpoint class"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := valid.Clone()
			test.mutate(&profile)
			err := profile.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGatewayProfilePreservesSlashInRawModelID(t *testing.T) {
	profile := Profile{
		ID: "openrouter", AdapterFamily: "openai-compatible",
		ModelIDs: []string{"anthropic/claude-sonnet-4"}, EndpointClass: EndpointGateway,
		SecretRefs: []string{"OPENROUTER_API_KEY"}, OptionsSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := profile.Clone().ModelIDs[0]; got != "anthropic/claude-sonnet-4" {
		t.Fatalf("raw gateway model ID = %q", got)
	}
}
