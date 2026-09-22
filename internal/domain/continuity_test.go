package domain

import (
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
	refs := make([]SourceRef, limits.SelectionRefs)
	for i := range refs {
		refs[i] = SourceRef{SessionID: "source", Kind: SourceKindMessage}
	}
	if err := (HistorySelection{SourceSessionID: "source", Refs: refs}).Validate(limits); err != nil {
		t.Fatalf("selection at exact maximum: %v", err)
	}
	refs = append(refs, SourceRef{SessionID: "source", Kind: SourceKindMessage})
	if err := (HistorySelection{SourceSessionID: "source", Refs: refs}).Validate(limits); err == nil {
		t.Fatal("accepted too many references")
	}
	files := make([]PresentFile, limits.PresentPaths)
	for i := range files {
		files[i] = PresentFile{Path: "a"}
	}
	if err := (PresentRequest{Files: files}).Validate(limits); err != nil {
		t.Fatalf("present set at exact maximum: %v", err)
	}
	if err := (PresentRequest{Files: append(files, PresentFile{Path: "b"})}).Validate(limits); err == nil {
		t.Fatal("accepted too many present paths")
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

func TestCanonicalHistoryScopeHashStable(t *testing.T) {
	first, err := NewAcceptedHistoryScope("destination", []SessionID{"b", "a", "b"})
	if err != nil {
		t.Fatalf("first scope: %v", err)
	}
	second, err := NewAcceptedHistoryScope("destination", []SessionID{"a", "b"})
	if err != nil {
		t.Fatalf("second scope: %v", err)
	}
	if first.ScopeHash != second.ScopeHash || strings.Join(sessionIDsToStrings(first.SourceSessionIDs), ",") != "a,b" {
		t.Fatalf("unstable canonical scope: %#v %#v", first, second)
	}
}

func TestHistoryPageKeepsRedactionAndTruncationIndependent(t *testing.T) {
	page := HistoryPage{
		Status:    HistoryStatusPartial,
		Redacted:  true,
		Truncated: true,
		Items: []HistoryItem{{
			Ref:       SourceRef{SessionID: "source", Kind: SourceKindMessage},
			Author:    HistoryAuthorUser,
			Text:      "safe",
			Redacted:  true,
			Truncated: true,
		}},
	}
	if err := page.Validate(DefaultContinuityLimits()); err != nil {
		t.Fatalf("mixed redaction/truncation page: %v", err)
	}
}

func TestEffectiveContinuityBudgetDoesNotExceedWireFrame(t *testing.T) {
	if got := DefaultContinuityLimits().Effective(4096).ResultPageBytes; got > 4096 {
		t.Fatalf("effective result budget %d exceeds wire frame", got)
	}
}

func sessionIDsToStrings(ids []SessionID) []string {
	values := make([]string, len(ids))
	for i := range ids {
		values[i] = string(ids[i])
	}
	return values
}
