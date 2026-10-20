package runtime

import (
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestModelCompletedV3AcrossSteps(t *testing.T) {
	m := newEventMapper("run-v3", 4096)
	pre, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.AgenticMessage]{
		Message: agenticTestMessage(schema.AgenticRoleTypeAssistant, "先读文件", testCalls(testCall("call-1", "read_file", "{}")), nil, nil),
	})
	if err != nil || len(pre) != 2 || pre[1].Type != domain.EventToolRequested {
		t.Fatalf("tool boundary: %+v %v", pre, err)
	}
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.AgenticMessage]{Message: testAssistant("你好🌱")})
	if err != nil || len(events) != 2 {
		t.Fatalf("final events: %+v %v", events, err)
	}
	completion := events[1]
	if completion.PayloadVersion != 3 || string(completion.Payload) != `{"byte_len":10}` {
		t.Fatalf("completion: %+v", completion)
	}
	if len(m.onTurnEnd()) != 0 {
		t.Fatal("duplicate completion")
	}
	if got := m.completedEvent(""); got.PayloadVersion != 3 || string(got.Payload) != `{"byte_len":0}` {
		t.Fatalf("empty completion: %+v", got)
	}
}

func TestCompletionProjectionHistoricalV2IgnoresDigest(t *testing.T) {
	for _, raw := range []string{`{"byte_len":3}`, `{"byte_len":3,"content_sha256":"wrong"}`, `{"byte_len":3,"content_sha256":null}`} {
		event := domain.RunEvent{Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: json.RawMessage(raw)}
		got, err := completedProjectionContent(event, "界")
		if err != nil || got != "界" {
			t.Fatalf("%s: %q %v", raw, got, err)
		}
	}
	for _, raw := range []string{`{}`, `{"byte_len":null}`, `{"byte_len":-1}`, `{"byte_len":1.5}`, `{"byte_len":0}`, `{"byte_len":3,"extra":true}`} {
		if _, err := completedProjectionContent(domain.RunEvent{Type: domain.EventModelCompleted, PayloadVersion: 3, Payload: json.RawMessage(raw)}, "界"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
