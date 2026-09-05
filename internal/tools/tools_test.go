package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
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

func TestBuiltinCatalogOmitsLegacyToolSearch(t *testing.T) {
	registry := Builtin(nil)
	for _, spec := range registry.Specs() {
		if spec.Name == "tool_search" {
			t.Fatal("builtin catalog must not register retired tool_search")
		}
	}
	if _, err := registry.Resolve([]string{"tool_search"}); err == nil {
		t.Fatal("registry must remain strict; legacy normalization belongs at config/settings input boundaries")
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

func TestWebFetchAndDownloadSpecs(t *testing.T) {
	if !NewWebFetch(nil).Spec().Readonly {
		t.Error("web_fetch must be readonly so policy auto-approves it like network_search")
	}
	if NewDownload(nil).Spec().Readonly {
		t.Error("download must be effectful so the gate can interrupt it (D-012)")
	}
	if _, ok := NewDownload(&stubDownload{}).(ProposalProvider); !ok {
		t.Error("download must implement ProposalProvider for the HITL flow")
	}
}

func TestWebFetchRejectsBadArgs(t *testing.T) {
	cases := map[string]string{
		"missing url":      `{}`,
		"empty url":        `{"url":"  "}`,
		"bad format":       `{"url":"https://example.com","format":"rtf"}`,
		"negative timeout": `{"url":"https://example.com","timeout":-1}`,
	}
	tool := NewWebFetch(nil)
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

func TestDownloadRejectsBadArgs(t *testing.T) {
	cases := map[string]string{
		"missing url":      `{"path":"x.bin"}`,
		"missing path":     `{"url":"https://example.com/f.zip"}`,
		"negative timeout": `{"url":"https://example.com","path":"x","timeout":-3}`,
	}
	tool := NewDownload(nil)
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

func TestDownloadPrepareProposalFallsBackToGeneric(t *testing.T) {
	tool := NewDownload(&stubDownload{})
	proposal, err := tool.(ProposalProvider).PrepareProposal(context.Background(), json.RawMessage(`{"url":"https://example.com/f.zip","path":"f.zip"}`))
	if err != nil {
		t.Fatalf("PrepareProposal: %v", err)
	}
	if proposal.Action != DownloadName || proposal.Target != "f.zip" || proposal.Preview == "" {
		t.Fatalf("unexpected generic proposal: %+v", proposal)
	}
}

type stubDownload struct{}

func (s *stubDownload) Download(_ context.Context, _ domain.RunID, _ DownloadRequest) (DownloadResult, error) {
	return DownloadResult{Path: "f.zip"}, nil
}
