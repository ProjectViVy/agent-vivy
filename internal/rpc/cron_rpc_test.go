package rpc

import (
	"encoding/json"
	"testing"
	"time"
)

func cronCapabilities(t *testing.T, handler Handler) []string {
	t.Helper()
	result, rpcErr := callControl(t, handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &init); err != nil {
		t.Fatal(err)
	}
	return init.Capabilities
}

func TestControlCronCapabilitiesRegistered(t *testing.T) {
	env := newControlTestEnv(t)
	caps := cronCapabilities(t, env.handler)
	want := []string{"cron.list", "cron.create", "cron.update", "cron.delete", "cron.trigger", "cron.stop"}
	for _, cap := range want {
		found := false
		for _, have := range caps {
			if have == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("capabilities missing %q", cap)
		}
	}
}

func TestControlCronCreateListUpdateDelete(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := json.RawMessage{}
	_ = ctx

	created, rpcErr := callControl(t, env.handler, "cron/create", map[string]any{
		"name": "morning brief",
		"schedule": map[string]any{
			"kind": "cron", "expr": "0 9 * * *", "tz": "Asia/Shanghai",
		},
		"payload": map[string]any{"kind": "agent_turn", "message": "summarize the inbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	job := decodeCronJob(t, created)
	if job.ID == "" || job.Name != "morning brief" {
		t.Fatalf("created job = %+v", job)
	}
	if job.Enabled != true || job.ComputedStatus != "scheduled" {
		t.Fatalf("created status = %+v", job)
	}
	if job.Schedule.TZ != "Asia/Shanghai" || job.Schedule.Kind != "cron" {
		t.Fatalf("created schedule = %+v", job.Schedule)
	}
	if job.State.NextRunAtMs == 0 {
		t.Fatalf("created job has no next run: %+v", job)
	}
	if job.SessionID != "" {
		t.Fatalf("session bound before first fire: %+v", job)
	}

	listed, rpcErr := callControl(t, env.handler, "cron/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listRaw, _ := json.Marshal(listed)
	var list struct {
		Jobs []cronJobResult `json:"jobs"`
	}
	if err := json.Unmarshal(listRaw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Jobs) != 1 || list.Jobs[0].ID != job.ID {
		t.Fatalf("list = %+v", list.Jobs)
	}

	updated, rpcErr := callControl(t, env.handler, "cron/update", map[string]any{
		"id":      job.ID,
		"name":    "morning brief",
		"enabled": false,
		"schedule": map[string]any{
			"kind": "cron", "expr": "0 9 * * *", "tz": "Asia/Shanghai",
		},
		"payload": map[string]any{"kind": "agent_turn", "message": "summarize the inbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	updatedJob := decodeCronJob(t, updated)
	if updatedJob.Enabled || updatedJob.ComputedStatus != "paused" || updatedJob.State.NextRunAtMs != 0 {
		t.Fatalf("updated job = %+v", updatedJob)
	}

	deleted, rpcErr := callControl(t, env.handler, "cron/delete", map[string]any{"id": job.ID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if raw, _ := json.Marshal(deleted); string(raw) != `{"deleted":true}` {
		t.Fatalf("delete result = %s", raw)
	}
	if _, rpcErr := callControl(t, env.handler, "cron/delete", map[string]any{"id": job.ID}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("double delete rpcErr = %+v, want %d", rpcErr, CodeNotFound)
	}
}

func TestControlCronCreateValidation(t *testing.T) {
	env := newControlTestEnv(t)

	cases := []struct {
		name   string
		params map[string]any
	}{
		{"empty name", map[string]any{
			"name":     "  ",
			"schedule": map[string]any{"kind": "every", "everyMs": 60000},
			"payload":  map[string]any{"message": "hi"},
		}},
		{"empty message", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "every", "everyMs": 60000},
			"payload":  map[string]any{"message": " "},
		}},
		{"bad expr", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "cron", "expr": "nope"},
			"payload":  map[string]any{"message": "hi"},
		}},
		{"bad tz", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "cron", "expr": "0 9 * * *", "tz": "Mars/Olympus"},
			"payload":  map[string]any{"message": "hi"},
		}},
		{"bad kind", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "weekly"},
			"payload":  map[string]any{"message": "hi"},
		}},
		{"unsupported payload kind", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "every", "everyMs": 60000},
			"payload":  map[string]any{"kind": "cleanup", "message": "hi"},
		}},
		{"zero interval", map[string]any{
			"name":     "job",
			"schedule": map[string]any{"kind": "every", "everyMs": 0},
			"payload":  map[string]any{"message": "hi"},
		}},
	}
	for _, tc := range cases {
		if _, rpcErr := callControl(t, env.handler, "cron/create", tc.params); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Errorf("%s: rpcErr = %+v, want InvalidParams", tc.name, rpcErr)
		}
	}
}

func TestControlCronTriggerConflictAndNotFound(t *testing.T) {
	env := newControlTestEnv(t)

	created, rpcErr := callControl(t, env.handler, "cron/create", map[string]any{
		"name":     "manual runner",
		"schedule": map[string]any{"kind": "every", "everyMs": 3600000},
		"payload":  map[string]any{"message": "one shot"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	job := decodeCronJob(t, created)

	// A disabled job can still be triggered manually.
	if _, rpcErr := callControl(t, env.handler, "cron/update", map[string]any{
		"id": job.ID, "name": job.Name, "enabled": false,
		"schedule": map[string]any{"kind": "every", "everyMs": 3600000},
		"payload":  map[string]any{"message": "one shot"},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}

	if _, rpcErr := callControl(t, env.handler, "cron/trigger", map[string]any{"id": job.ID}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "cron/trigger", map[string]any{"id": job.ID}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("second trigger rpcErr = %+v, want %d", rpcErr, CodeConflict)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, rpcErr := callControl(t, env.handler, "cron/trigger", map[string]any{"id": job.ID})
		if rpcErr == nil {
			break // previous run settled
		}
		if rpcErr.Code != CodeConflict {
			t.Fatalf("re-trigger rpcErr = %+v", rpcErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, rpcErr := callControl(t, env.handler, "cron/trigger", map[string]any{"id": "cron_missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing trigger rpcErr = %+v, want %d", rpcErr, CodeNotFound)
	}
	if _, rpcErr := callControl(t, env.handler, "cron/stop", map[string]any{"id": "cron_missing"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("stop missing rpcErr = %+v, want %d", rpcErr, CodeConflict)
	}
}

// decodeCronJob unwraps {job: ...} into the wire struct.
func decodeCronJob(t *testing.T, result any) cronJobResult {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		Job cronJobResult `json:"job"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatal(err)
	}
	return wrapper.Job
}
