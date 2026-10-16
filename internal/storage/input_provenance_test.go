package storage

import (
	"agent-vivy/internal/domain"
	"testing"
)

func TestIssue58InputProvenanceDecoderPresence(t *testing.T) {
	for _, raw := range []string{`{}`, `{"input_provenance":null}`, `{"input_provenance":{"content_origin":"original"}}`, `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":null}}`, `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":"false"}}`, `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":false,"extra":1}}`} {
		if _, err := DecodeModelInputProvenance(domain.RunEvent{PayloadVersion: 4, Payload: []byte(raw)}); err == nil {
			t.Errorf("accepted malformed v4: %s", raw)
		}
	}
	for _, origin := range []string{"original", "notebook", "unknown"} {
		raw := `{"input_provenance":{"content_origin":"` + origin + `","exclude_automatic_ingest":false}}`
		got, err := DecodeModelInputProvenance(domain.RunEvent{PayloadVersion: 4, Payload: []byte(raw)})
		if err != nil || got.ContentOrigin != origin {
			t.Fatalf("explicit false lost: %+v %v", got, err)
		}
	}
	got, err := DecodeModelInputProvenance(domain.RunEvent{PayloadVersion: 3, Payload: []byte(`{}`)})
	if err != nil || got.ContentOrigin != "unknown" {
		t.Fatalf("legacy recertified: %+v %v", got, err)
	}
}

func TestIssue58InputProvenanceUnknownDominates(t *testing.T) {
	got := MergeInputProvenance(InputProvenance{ContentOrigin: "notebook", ExcludeAutomaticIngest: true}, InputProvenance{ContentOrigin: "unknown"})
	if got.ContentOrigin != "unknown" || got.ExcludeAutomaticIngest {
		t.Fatalf("excluded hid unknown: %+v", got)
	}
}

func TestIssue58ClassifyInputProvenance(t *testing.T) {
	for _, tc := range []struct {
		origin                                string
		excluded, wantExcluded, wantAvailable bool
	}{{"original", false, false, true}, {"notebook", true, true, true}, {"", false, false, false}, {"unknown", true, false, false}, {"other", false, false, false}} {
		excluded, available := ClassifyInputProvenance(tc.origin, tc.excluded)
		if excluded != tc.wantExcluded || available != tc.wantAvailable {
			t.Fatalf("classify %+v = %t,%t", tc, excluded, available)
		}
	}
}

func TestIssue58UnrecognizedOriginDominatesExclusion(t *testing.T) {
	p := MergeInputProvenance(InputProvenance{ContentOrigin: "future"}, InputProvenance{ContentOrigin: "notebook", ExcludeAutomaticIngest: true})
	if p.ContentOrigin != "unknown" || p.ExcludeAutomaticIngest {
		t.Fatalf("recertified unrecognized origin: %+v", p)
	}
}
