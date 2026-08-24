package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateArgsRejectsManifestViolations(t *testing.T) {
	spec := NewEchoInfo().Spec()
	for name, args := range map[string]string{
		"missing required": `{}`,
		"unknown field":    `{"text":"ok","extra":"nope"}`,
		"wrong type":       `{"text":42}`,
		"trailing value":   `{"text":"ok"}{}`,
	} {
		if err := ValidateArgs(spec, json.RawMessage(args)); err == nil {
			t.Errorf("%s: want schema error", name)
		}
	}
}

func TestValidateArgsAllowsEmptyObjectForNoParams(t *testing.T) {
	if err := ValidateArgs(NewListNotes(nil).Spec(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("empty object: %v", err)
	}
	if err := ValidateArgs(NewListNotes(nil).Spec(), nil); err != nil {
		t.Fatalf("empty args: %v", err)
	}
}

func TestSelectorChoosesOnlyRelevantTools(t *testing.T) {
	ts := []Tool{NewEchoInfo(), NewWriteNote(nil), NewListNotes(nil), NewReadNote(nil)}

	cases := []struct {
		request string
		want    []string
	}{
		{request: "save this as a note", want: []string{WriteNoteName}},
		{request: "list my notes", want: []string{ListNotesName}},
		{request: "read note_abc123", want: []string{ReadNoteName}},
		{request: "echo hello", want: []string{EchoInfoName}},
		{request: "hello vivy", want: nil},
	}
	for _, tc := range cases {
		got := NewSelector(ts).Select(tc.request).Names()
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("Select(%q) = %v, want %v", tc.request, got, tc.want)
		}
	}
}

func TestEchoInfoSpec(t *testing.T) {
	spec := NewEchoInfo().Spec()
	if spec.Name != EchoInfoName {
		t.Errorf("name = %q", spec.Name)
	}
	if !spec.Readonly {
		t.Error("echo_info must be readonly to auto-execute (D-012)")
	}
}

func TestEchoInfoRun(t *testing.T) {
	out, err := NewEchoInfo().InvokableRun(context.Background(),
		json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if out != "hello" {
		t.Errorf("out = %q, want verbatim echo", out)
	}
}

func TestEchoInfoRejectsBadArgs(t *testing.T) {
	cases := map[string]string{
		"missing text":   `{}`,
		"empty text":     `{"text":""}`,
		"unknown field":  `{"text":"x","extra":1}`,
		"not an object":  `"plain string"`,
		"malformed json": `{`,
		"oversized text": `{"text":"` + strings.Repeat("a", echoTextLimit+1) + `"}`,
	}
	tool := NewEchoInfo()
	for name, args := range cases {
		_, err := tool.InvokableRun(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Errorf("%s: want error, got nil", name)
			continue
		}
		var argErr *ArgError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: error %T is not a structured *ArgError", name, err)
		}
	}
}

func TestWriteNoteSpec(t *testing.T) {
	spec := NewWriteNote(nil).Spec()
	if spec.Name != WriteNoteName {
		t.Errorf("name = %q", spec.Name)
	}
	if spec.Readonly {
		t.Error("write_note must be effectful so the gate can interrupt it (D-012)")
	}
}

func TestWriteNoteRejectsBadArgs(t *testing.T) {
	cases := map[string]string{
		"missing content": `{}`,
		"empty content":   `{"content":""}`,
		"unknown field":   `{"content":"x","extra":1}`,
		"not an object":   `"plain string"`,
		"oversized":       `{"content":"` + strings.Repeat("a", noteContentLimit+1) + `"}`,
	}
	tool := NewWriteNote(&memNotes{})
	for name, args := range cases {
		_, err := tool.InvokableRun(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Errorf("%s: want error, got nil", name)
			continue
		}
		var argErr *ArgError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: error %T is not a structured *ArgError", name, err)
		}
	}
}

func TestRegistryResolve(t *testing.T) {
	reg := Builtin(&memNotes{})

	got, err := reg.Resolve([]string{EchoInfoName, WriteNoteName})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 2 || got[0].Spec().Name != EchoInfoName || got[1].Spec().Name != WriteNoteName {
		t.Errorf("Resolve = %+v", got)
	}

	// The notes trio is registered under its names (MA-3).
	if _, err := reg.Resolve([]string{ListNotesName, ReadNoteName}); err != nil {
		t.Errorf("notes tools must resolve: %v", err)
	}

	if _, err := reg.Resolve([]string{"send_email"}); err == nil {
		t.Error("unknown tool must be a startup error")
	}

	if got, err := reg.Resolve(nil); err != nil || len(got) != 0 {
		t.Errorf("Resolve(nil) = %v, %v", got, err)
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate registration must panic")
		}
	}()
	NewRegistry(NewEchoInfo(), NewEchoInfo())
}
