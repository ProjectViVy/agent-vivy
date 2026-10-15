package runtime

import (
	"agent-vivy/internal/storage"
	"strings"
	"testing"
)

func TestIssue58RecapReceiptCodec(t *testing.T) {
	b := storage.SubjectHistoryBinding{SubjectID: "subject", DomainID: "domain"}
	d := storage.SubjectDisposition{SessionID: "view", MarkerID: 1, Marker: storage.SessionTruncation{SessionID: "view", Reason: storage.TruncationRewind, CutoffMessageID: "cut", TailMessageID: "tail"}, Targets: []storage.SubjectDispositionTarget{{InteractionID: "run:1", Sequence: 1, SourceSessionID: "source", RunID: "run", SourceDigest: strings.Repeat("a", 64), MessageIDs: []string{"b", "a"}, ViewMessageIDs: []string{"vb", "va"}}}}
	r := recapReceipt{Version: 1, ProjectionPolicy: 1, Binding: b, Range: recapReceiptRange{After: 0, Through: 1, ExactOriginalsDigest: strings.Repeat("a", 64)}, Dispositions: recapReceiptDispositions{Digest: strings.Repeat("b", 64)}, CorrectionRefs: []storage.SubjectDisposition{d}, CorrectionEvidenceDigest: strings.Repeat("c", 64)}
	encoded, err := encodeRecapReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeRecapReceipt(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.CorrectionRefs[0].Targets[0].MessageIDs[0] != "a" || got.CorrectionRefs[0].Targets[0].ViewMessageIDs[0] != "va" {
		t.Fatal("lost pair association")
	}
	encoded2, err := encodeRecapReceipt(got)
	if err != nil || encoded2 != encoded {
		t.Fatal("not canonical")
	}
	for _, bad := range []string{"", `{"v":2}`, strings.Replace(encoded, `"projection_policy":1`, `"projection_policy":2`, 1), strings.Replace(encoded, `"projection_policy":1,`, "", 1), strings.Replace(encoded, `"v":1`, `"v":1,"unknown":1`, 1), encoded + `{}`} {
		if _, err := decodeRecapReceipt(bad); err == nil {
			t.Fatalf("accepted unavailable certificate %s", bad)
		}
	}
	r.CorrectionRefs[0].Targets[0].ViewMessageIDs = nil
	if _, err := encodeRecapReceipt(r); err == nil {
		t.Fatal("accepted unaligned pairs")
	}
}

func TestIssue58RecapReceiptActualUsedSubset(t *testing.T) {
	d := storage.SubjectDisposition{SessionID: "view", MarkerID: 1, Marker: storage.SessionTruncation{SessionID: "view", Reason: storage.TruncationRewind}, Targets: []storage.SubjectDispositionTarget{{InteractionID: "run:1", Sequence: 1, RunID: "run", SourceSessionID: "source", SourceDigest: strings.Repeat("a", 64), MessageIDs: []string{"a", "b"}, ViewMessageIDs: []string{"va", "vb"}}}}
	snapshot := storage.SubjectContextSnapshot{Dispositions: []storage.SubjectDisposition{d}}
	used := d
	used.Targets = append([]storage.SubjectDispositionTarget(nil), d.Targets...)
	used.Targets[0].MessageIDs = []string{"b"}
	used.Targets[0].ViewMessageIDs = []string{"vb"}
	if err := validateRecapCorrectionReferences([]storage.SubjectDisposition{used}, snapshot); err != nil {
		t.Fatal(err)
	}
	used.Targets[0].ViewMessageIDs = []string{"va"}
	if err := validateRecapCorrectionReferences([]storage.SubjectDisposition{used}, snapshot); err == nil {
		t.Fatal("accepted crossed canonical/view pair")
	}
	used.Targets[0].ViewMessageIDs = []string{"vb"}
	used.Marker.WorkSeq++
	if err := validateRecapCorrectionReferences([]storage.SubjectDisposition{used}, snapshot); err == nil {
		t.Fatal("accepted marker metadata not actually captured")
	}
}
