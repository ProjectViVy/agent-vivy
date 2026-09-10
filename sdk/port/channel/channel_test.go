package channel

import (
	"context"
	"testing"
)

type testInstance struct{}

func (testInstance) Start(context.Context) error { return nil }
func (testInstance) Stop(context.Context) error  { return nil }
func (testInstance) Send(context.Context, OutboundMessage) ([]string, error) {
	return []string{"id"}, nil
}

func TestChannelInstancePreservesTypedEnvelope(t *testing.T) {
	var instance Instance = testInstance{}
	ids, err := instance.Send(context.Background(), OutboundMessage{ChatID: "chat", Parts: []Part{{Kind: PartText, Text: "hello"}}})
	if err != nil || len(ids) != 1 || ids[0] != "id" {
		t.Fatalf("Send = %v, %v", ids, err)
	}
}
