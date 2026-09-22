package domain

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestContinuityLimitsExactBounds(t *testing.T) {
	limits := DefaultContinuityLimits()
	if err := (HistorySearchRequest{Query: strings.Repeat("q", limits.SearchQueryBytes), Limit: limits.SearchPageMax}).Validate(limits); err != nil {
		t.Fatalf("search at exact maximum: %v", err)
	}
	if err := (HistorySearchRequest{Query: strings.Repeat("q", limits.SearchQueryBytes+1)}).Validate(limits); err == nil {
		t.Fatal("accepted oversized query")
	}
	if err := (HistoryReadRequest{ReferenceID: "reference", Limit: limits.ReadPageMax}).Validate(limits); err != nil {
		t.Fatalf("read at exact maximum: %v", err)
	}
	if err := (HistoryReadRequest{ReferenceID: "reference", Limit: limits.ReadPageMax + 1}).Validate(limits); err == nil {
		t.Fatal("accepted oversized read page")
	}
	if err := (PresentRequest{Files: []PresentFile{{Path: "a", Description: strings.Repeat("d", limits.DescriptionBytes)}}}).Validate(limits); err != nil {
		t.Fatalf("description at exact maximum: %v", err)
	}
	if err := (PresentRequest{Files: []PresentFile{{Path: "a", Description: strings.Repeat("d", limits.DescriptionBytes+1)}}}).Validate(limits); err == nil {
		t.Fatal("accepted oversized description")
	}
	if err := (DeliveryReadRequest{ItemID: "item", ExpectedDigest: "digest", Length: limits.BinaryPageBytes}).Validate(limits); err != nil {
		t.Fatalf("binary page at exact maximum: %v", err)
	}
	if err := (DeliveryReadRequest{ItemID: "item", ExpectedDigest: "digest", Length: limits.BinaryPageBytes + 1}).Validate(limits); err == nil {
		t.Fatal("accepted oversized binary page")
	}
}

func TestContinuitySelectorValidation(t *testing.T) {
	limits := DefaultContinuityLimits()
	selection := HistorySelection{SourceSessionID: "source", Refs: []SourceRef{{SessionID: "source", Kind: SourceKindMessage}}, RunRange: &HistoryRunRange{RunID: "run", FromSeq: 1, ToSeq: 1}}
	if err := selection.Validate(limits); err == nil {
		t.Fatal("accepted both history selectors")
	}
	if err := (HistoryReadRequest{}).Validate(limits); err == nil {
		t.Fatal("accepted missing read selector")
	}
	if err := (HistoryTraceRequest{SourceRef: &SourceRef{SessionID: "source", Kind: SourceKindMessage}, ReferenceID: "reference"}).Validate(limits); err == nil {
		t.Fatal("accepted multiple trace selectors")
	}
}

func TestContinuityRejectsInvalidUTF8AndUnknownValues(t *testing.T) {
	limits := DefaultContinuityLimits()
	invalid := string([]byte{0xff})
	if err := (HistorySearchRequest{Query: invalid}).Validate(limits); err == nil {
		t.Fatal("accepted invalid UTF-8 query")
	}
	if HistoryStatus("invented").Valid() {
		t.Fatal("unknown history status is valid")
	}
	if SourceKind("invented").Valid() {
		t.Fatal("unknown source kind is valid")
	}
	if err := (HistorySearchRequest{Kinds: []string{"invented"}}).Validate(limits); err == nil {
		t.Fatal("accepted unknown history kind")
	}
}

func TestContinuityCanonicalHistoryScopeHash(t *testing.T) {
	accepted, err := NewAcceptedHistoryScope("destination", []SessionID{"b", "a", "b"})
	if err != nil {
		t.Fatalf("new scope: %v", err)
	}
	if got, want := accepted.ScopeHash, "6d9aef6d4fca8bc36a3ac48914ebc16f64ba3b6d2cf8624019f3e436a45ed45e"; got != want {
		t.Fatalf("scope hash = %q, want %q", got, want)
	}
	if err := accepted.Validate(); err != nil {
		t.Fatalf("canonical scope rejected: %v", err)
	}
	unsorted := accepted
	unsorted.SourceSessionIDs = []SessionID{"b", "a"}
	if err := unsorted.Validate(); err == nil {
		t.Fatal("accepted non-canonical source session order")
	}
	duplicate := accepted
	duplicate.SourceSessionIDs = []SessionID{"a", "a"}
	if err := duplicate.Validate(); err == nil {
		t.Fatal("accepted duplicate source session ID")
	}
}

func TestContinuityHistoryPageRedactionAndTruncationIndependent(t *testing.T) {
	for _, flags := range []struct {
		name                string
		redacted, truncated bool
	}{
		{"neither", false, false},
		{"redacted", true, false},
		{"truncated", false, true},
		{"both", true, true},
	} {
		t.Run(flags.name, func(t *testing.T) {
			page := HistoryPage{
				Status: HistoryStatusPartial, Redacted: flags.redacted, Truncated: flags.truncated,
				Items: []HistoryItem{{
					Ref: SourceRef{SessionID: "source", Kind: SourceKindMessage}, Author: HistoryAuthorUser, Text: "safe",
					Redacted: flags.redacted, Truncated: flags.truncated,
				}},
			}
			if err := page.Validate(DefaultContinuityLimits()); err != nil {
				t.Fatalf("page rejected: %v", err)
			}
		})
	}
}

func TestContinuityAggregateLimits(t *testing.T) {
	limits := DefaultContinuityLimits()
	references := make([]ContextReference, limits.ReferencesPerTask)
	for i := range references {
		references[i] = validContextReference(string(rune('a' + i)), strings.Repeat("x", 8000))
	}
	if err := ValidateContextReferences(references[:limits.ReferencesPerTask-1], limits); err != nil {
		t.Fatalf("reference aggregate at bounded size: %v", err)
	}
	if err := ValidateContextReferences(references, limits); err == nil {
		t.Fatal("accepted oversized reference aggregate")
	}

	files := make([]Deliverable, 4)
	for i := range files {
		files[i] = validDeliverable(string(rune('a' + i)), limits.PresentFileBytes)
	}
	set := DeliverySet{ID: "set", SessionID: "session", RunID: "run", ToolCallID: "call", Items: files, Status: DeliveryStatusOK}
	if err := set.Validate(limits); err != nil {
		t.Fatalf("delivery aggregate at exact maximum: %v", err)
	}
	set.Items = append(set.Items, validDeliverable("e", 1))
	if err := set.Validate(limits); err == nil {
		t.Fatal("accepted oversized delivery aggregate")
	}
}

func TestContinuityDeliveryChunkPageBound(t *testing.T) {
	limits := DefaultContinuityLimits()
	data := strings.Repeat("x", limits.BinaryPageBytes)
	chunk := DeliveryChunk{TransferID: "transfer", ItemID: "item", Digest: "digest", DataBase64: base64.StdEncoding.EncodeToString([]byte(data))}
	if err := chunk.Validate(limits); err != nil {
		t.Fatalf("chunk at exact maximum: %v", err)
	}
	chunk.DataBase64 = base64.StdEncoding.EncodeToString([]byte(data + "x"))
	if err := chunk.Validate(limits); err == nil {
		t.Fatal("accepted oversized chunk")
	}
}

func TestContinuityEffectiveBudgetDoesNotExceedWireFrame(t *testing.T) {
	if got := DefaultContinuityLimits().Effective(4096).ResultPageBytes; got > 4096 {
		t.Fatalf("effective result budget %d exceeds wire frame", got)
	}
}

func validContextReference(id, text string) ContextReference {
	return ContextReference{
		ID: id, DestinationSessionID: "destination", DestinationRunID: "run", SourceSessionID: "source",
		SourceWorkspace: "workspace", Digest: "digest", Origin: "user_selection",
		Items: []HistoryItem{{Ref: SourceRef{SessionID: "source", Kind: SourceKindMessage}, Author: HistoryAuthorUser, Text: text}},
	}
}

func validDeliverable(path string, size int64) Deliverable {
	return Deliverable{
		ID: path, SessionID: "session", RunID: "run", WorkspaceID: "workspace", Path: path, Name: path,
		SHA256: "digest", MediaType: "text/plain", Size: size, OriginToolCallID: "call",
	}
}
