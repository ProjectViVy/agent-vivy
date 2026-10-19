package app

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/port/observer"
	"context"
	"path/filepath"
	"testing"
)

func TestIssue58AutomaticIngestProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, request         string
		version               int
		excluded, unavailable bool
	}{
		{"original", `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":false}}`, 4, false, false},
		{"notebook", `{"input_provenance":{"content_origin":"notebook","exclude_automatic_ingest":true}}`, 4, true, false},
		{"unknown", `{"input_provenance":{"content_origin":"unknown","exclude_automatic_ingest":false}}`, 4, false, true},
		{"missing-bool", `{"input_provenance":{"content_origin":"notebook"}}`, 4, false, true},
		{"legacy", `{}`, 3, false, true},
		{"invalid-args", `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":false}}`, 4, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "ingest.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			if err := b.CreateSession(ctx, domain.Session{ID: "s", CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if err := b.CreateRun(ctx, domain.Run{ID: "r", SessionID: "s", Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Append(ctx, storage.Commit{RunID: "r", Events: []domain.RunEvent{{Type: domain.EventModelRequest, PayloadVersion: tc.version, Payload: []byte(tc.request)}}}); err != nil {
				t.Fatal(err)
			}
			if tc.name == "invalid-args" {
				if _, err := b.Append(ctx, storage.Commit{RunID: "r", Events: []domain.RunEvent{{Type: domain.EventToolRequested, PayloadVersion: 2, Payload: []byte(`{"original_arguments_invalid_utf8":true,"original_arguments_bytes":"/w==","tool_call_id":"invalid","tool_name":"test","args":{}}`)}}}); err != nil {
					t.Fatal(err)
				}
			}
			excluded, err := ingestExclusionPredicate(b)(ctx, "r")
			if tc.unavailable {
				// Unknown provenance is skipped (cursor advances), never fed to
				// memory and never retried forever.
				if err != nil || !excluded {
					t.Fatalf("unknown automatic ingest must be skipped: %t %v", excluded, err)
				}
			} else if err != nil || excluded != tc.excluded {
				t.Fatalf("ingest decision %t %v", excluded, err)
			}
		})
	}
}

type issue58IngestReceiver struct{ delivered int }

func (*issue58IngestReceiver) ID() string { return "fixture/issue58-memory" }
func (r *issue58IngestReceiver) ObserveRun(context.Context, observer.RunEvent) error {
	r.delivered++
	return nil
}

func TestIssue58InvalidArgumentsSkipObserverDelivery(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "original-delivers", true: "invalid-skipped"}[invalid], func(t *testing.T) {
			ctx := context.Background()
			b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "delivery.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			if err := b.CreateSession(ctx, domain.Session{ID: "delivery-session", CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if err := b.CreateRun(ctx, domain.Run{ID: "delivery-run", SessionID: "delivery-session", Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			events := []domain.RunEvent{{Type: domain.EventModelRequest, PayloadVersion: 4, Payload: []byte(`{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":false}}`)}}
			terminal := domain.EventRunCompleted
			if invalid {
				events = append(events, domain.RunEvent{Type: domain.EventToolRequested, PayloadVersion: 2, Payload: []byte(`{"original_arguments_invalid_utf8":true,"original_arguments_bytes":"/w==","tool_call_id":"invalid","tool_name":"test","args":{}}`)})
				terminal = domain.EventRunFailed
			}
			events = append(events, domain.RunEvent{Type: terminal, PayloadVersion: 2, Payload: []byte(`{"result":"committed evidence"}`)})
			if _, err := b.Append(ctx, storage.Commit{RunID: "delivery-run", Events: events}); err != nil {
				t.Fatal(err)
			}
			receiver := &issue58IngestReceiver{}
			host, err := observerhost.New(observerhost.Config{Journal: b, Cursors: b.Snapshot(), RunSubscriptions: []observerhost.RunSubscription{{Provider: receiver, EventTypes: []string{string(domain.EventRunCompleted), string(domain.EventRunFailed)}, AllowedPayloadFields: []string{"result"}, ExcludeRun: ingestExclusionPredicate(b)}}})
			if err != nil {
				t.Fatal(err)
			}
			err = host.DeliverRun(ctx, "delivery-run")
			if invalid {
				if receiver.delivered != 0 || err != nil {
					t.Fatalf("invalid evidence delivered or retried by automatic observer %d %v", receiver.delivered, err)
				}
				// The cursor advanced: a second delivery is a no-op, not a retry.
				if err := host.DeliverRun(ctx, "delivery-run"); err != nil || receiver.delivered != 0 {
					t.Fatalf("skipped run was redelivered %d %v", receiver.delivered, err)
				}
			} else if err != nil || receiver.delivered != 1 {
				t.Fatalf("original observer delivery missing %d %v", receiver.delivered, err)
			}
		})
	}
}
