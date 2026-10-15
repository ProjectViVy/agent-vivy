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
