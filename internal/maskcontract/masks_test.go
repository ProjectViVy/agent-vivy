package maskcontract

import (
	"errors"
	"strings"
	"testing"
)

const testOperationID = "00000000-0000-4000-8000-000000000001"

func TestMaskNormalizePreservesLiteralTemplate(t *testing.T) {
	in := CreateRequest{
		OperationID: testOperationID,
		Name:        "  Writer  ",
		Body:        "{system}\r\n{{persona}}",
	}
	got, err := NormalizeCreate(in)
	if err != nil || got.Name != "Writer" || got.Body != "{system}\n{{persona}}" {
		t.Fatalf("literal normalization failed: %+v: %v", got, err)
	}
}

func TestMaskNormalizeCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "blank", body: " \t\n", want: "body"},
		{name: "nul", body: "ok\x00body", want: "control"},
		{name: "del", body: "ok\x7fbody", want: "control"},
		{name: "invalid utf8", body: string([]byte{0xc3, 0x28}), want: "UTF-8"},
		{name: "multibyte overflow", body: strings.Repeat("界", MaxBodyBytes/len("界")+1), want: "exceeds"},
		{name: "control", body: "ok\vbody", want: "control"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeCreate(CreateRequest{OperationID: testOperationID, Name: "Mask", Body: test.body})
			if err == nil || !strings.Contains(err.Error(), CodeInvalidMask) {
				t.Fatalf("error = %v, want safe invalid_mask", err)
			}
			cause := errors.Unwrap(err)
			if cause == nil || !strings.Contains(strings.ToLower(cause.Error()), strings.ToLower(test.want)) {
				t.Fatalf("cause = %v, want %q", cause, test.want)
			}
		})
	}
}

func TestMaskNormalizeLineEndingsAndLimits(t *testing.T) {
	got, err := NormalizeCreate(CreateRequest{
		OperationID: testOperationID,
		Name:        "Mask",
		Body:        "first\r\nsecond\rthird",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "first\nsecond\nthird" {
		t.Fatalf("body = %q, want LF-normalized body", got.Body)
	}
	if _, err := NormalizeCreate(CreateRequest{OperationID: testOperationID, Name: strings.Repeat("界", 64), Body: "body"}); err == nil {
		t.Fatal("multibyte name over byte limit was accepted")
	}
	if _, err := NormalizeCreate(CreateRequest{OperationID: testOperationID, Name: "Mask", Description: strings.Repeat("x", MaxDescriptionBytes+1), Body: "body"}); err == nil {
		t.Fatal("description over byte limit was accepted")
	}
	if _, err := NormalizeCreate(CreateRequest{OperationID: "not-a-uuid", Name: "Mask", Body: "body"}); err == nil {
		t.Fatal("invalid operation UUID was accepted")
	}
}

func TestMaskNormalizeReservedIDCollisions(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "programmer", id: BuiltinProgrammerID},
		{name: "researcher", id: BuiltinResearcherID},
		{name: "writer", id: BuiltinWriterID},
		{name: "arbitrary builtin", id: "builtin/other"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalizeUpdate(UpdateRequest{ID: test.id, ExpectedRevision: 1, Name: "Mask", Body: "body"}); err == nil {
				t.Fatalf("reserved id %q was accepted", test.id)
			}
			if _, err := NormalizeDelete(DeleteRequest{ID: test.id, ExpectedRevision: 1}); err == nil {
				t.Fatalf("reserved id %q was accepted for delete", test.id)
			}
		})
	}
	if err := ValidateCustomID("custom/00000000-0000-4000-8000-000000000001"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCustomID("custom/00000000-0000-4000-8000-00000000000z"); err == nil {
		t.Fatal("invalid custom UUID was accepted")
	}
}

func TestMaskNormalizeSelectionAndList(t *testing.T) {
	if got, err := NormalizeList(ListRequest{}); err != nil || got.Limit != DefaultListLimit {
		t.Fatalf("default list limit = %+v, err=%v", got, err)
	}
	for _, limit := range []int{-1, MaxListLimit + 1} {
		if _, err := NormalizeList(ListRequest{Limit: limit}); err == nil {
			t.Fatalf("limit %d was accepted", limit)
		}
	}
	if _, err := NormalizeSelection(SetSelectionRequest{SessionID: "session", MaskID: BuiltinWriterID, ExpectedRevision: -1}); err == nil {
		t.Fatal("negative selection revision was accepted")
	}
	if _, err := NormalizeSelection(SetSelectionRequest{SessionID: "session", MaskID: "", ExpectedRevision: 0}); err != nil {
		t.Fatal(err)
	}
}

func TestMaskErrorRedactsCauseAndUnwraps(t *testing.T) {
	cause := errors.New("body=secret SQL=credential")
	err := NewReferenceError(CodeMaskInUse, 3, cause)
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "SQL") {
		t.Fatalf("error leaked cause: %v", err)
	}
	if !errors.Is(err, cause) || err.ReferenceCount != 3 || err.Code != CodeMaskInUse {
		t.Fatalf("error metadata/cause mismatch: %+v", err)
	}
	if got := NewError("arbitrary_internal_code", cause).Code; got != CodeInvalidMask {
		t.Fatalf("unknown code = %q, want %q", got, CodeInvalidMask)
	}
}
