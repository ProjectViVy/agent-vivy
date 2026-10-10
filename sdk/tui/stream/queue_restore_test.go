package stream

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestQueueDequeueInterpretRetainsCompleteTurnForPrivateEditor(t *testing.T) {
	raw := json.RawMessage(`{"reason":"aborted","track":"follow_up","text":"restore","turn":{"id":"q1","thinking":"high","attachments":[{"name":"capture.png","data":"aW1hZ2U="}]}}`)
	notice := Interpret(Event{Type: "turn.dequeued", Payload: raw})
	if notice.Kind != "queue_dequeued" || notice.QueueReason != "aborted" || !bytes.Contains(notice.QueueTurn, []byte(`"thinking":"high"`)) || !bytes.Contains(notice.QueueTurn, []byte(`"data":"aW1hZ2U="`)) {
		t.Fatalf("full queue DTO lost in normalization: %+v", notice)
	}
}
