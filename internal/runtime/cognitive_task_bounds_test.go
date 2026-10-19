package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
)

func TestCognitiveInferencePreservesCompleteBoundedRequest(t *testing.T) {
	input, err := json.Marshal(map[string]string{"fact": strings.Repeat("记忆事实", 420)})
	if err != nil {
		t.Fatal(err)
	}
	req := laputaevolution.ModelRequest{
		Stage:        laputaevolution.StageReflect,
		Prompt:       "Review the supplied evidence.",
		InputJSON:    input,
		OutputSchema: json.RawMessage(`{"type":"object","required":["candidates"]}`),
	}
	task := cognitiveInferTask(req)
	if !strings.Contains(task, string(input)) {
		t.Fatal("admitted child task lost the complete source JSON")
	}
	if !strings.HasSuffix(task, string(req.OutputSchema)) {
		t.Fatal("admitted child task lost the output schema")
	}
	if !utf8.ValidString(task) || len(task) > maxChildTaskBytes {
		t.Fatal("request is not within the existing native child task contract")
	}
}
