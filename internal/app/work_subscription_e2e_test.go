package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

// Durable commits must also reach a subscriber attached before the commit.
// Replay-only tests cannot catch a missing runtime-to-RPC WorkSink binding.
func TestAppPublishesLiveWorkToControlSubscriber(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "work-subscription-test-dummy")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	cfg, _ := planGoalTestConfig(t, []string{tools.SubmitPlanName})
	a := openPlanGoalTestApp(t, cfg)
	capture := &dn0Capture{t: t}
	client, err := a.DialControl(context.Background(), capture)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	session := callControl(t, client, "session/create", map[string]any{"title": "Live Work"})
	sessionID := session["id"].(string)
	subscription := callControl(t, client, "session/work/subscribe", map[string]any{"session_id": sessionID, "after_seq": 0})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "expected_version": 0, "request_id": "live-plan-entry"})
	notification := capture.waitForNotification(time.Second, func(record dn0Record) bool {
		return record.Method == "session/work/event"
	})
	var envelope struct {
		SubscriptionID string `json:"subscription_id"`
		Event          struct {
			Seq  int64  `json:"seq"`
			Kind string `json:"kind"`
		} `json:"event"`
	}
	if err := json.Unmarshal(notification.Params, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SubscriptionID != subscription["subscription_id"] || envelope.Event.Seq != 1 || envelope.Event.Kind != "plan.entered" {
		t.Fatalf("unexpected live work notification: %s", notification.Params)
	}
}
