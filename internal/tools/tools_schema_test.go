package tools

import (
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

func TestValidateArgsHonorsFullJSONSchema(t *testing.T) {
	spec := domain.ToolSpec{
		Name: "schema.tool",
		Schema: json.RawMessage(`{
			"$schema":"https://json-schema.org/draft/2020-12/schema",
			"type":"object",
			"properties":{
				"mode":{"type":"string","enum":["fast","safe"]},
				"items":{"type":"array","minItems":1,"items":{"type":"integer","minimum":1}},
				"config":{"type":"object","required":["enabled"],"properties":{"enabled":{"type":"boolean"}}}
			},
			"required":["mode","items","config"],
			"additionalProperties":false
		}`),
	}
	valid := json.RawMessage(`{"mode":"safe","items":[1,2],"config":{"enabled":true}}`)
	if err := ValidateArgs(spec, valid); err != nil {
		t.Fatalf("valid nested schema rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"mode":"unsafe","items":[1],"config":{"enabled":true}}`),
		json.RawMessage(`{"mode":"safe","items":[],"config":{"enabled":true}}`),
		json.RawMessage(`{"mode":"safe","items":[0],"config":{"enabled":true}}`),
		json.RawMessage(`{"mode":"safe","items":[1],"config":{}}`),
		json.RawMessage(`{"mode":"safe","items":[1],"config":{"enabled":true},"extra":true}`),
	} {
		if err := ValidateArgs(spec, args); err == nil {
			t.Fatalf("invalid nested schema accepted: %s", args)
		}
	}
}
