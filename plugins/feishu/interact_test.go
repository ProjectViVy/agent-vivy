package feishu

// Interaction-capability coverage (contract §1/§12, 2026-09-15): every
// text part goes out as a schema-2.0 markdown card with the 11310
// plain-text fallback, edits Patch card content, deletes remove sent
// messages, the placeholder is the fixed copy as a card, and the ack
// reaction draws from the configured pool and withdraws by reaction id.
// All of it runs against the loopback larkStub from plugin_test.go.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	plugin "agent-vivy/sdk/port/channel"
)

// feishuSettings points a fake env at the stub with optional ack_emojis.
func feishuSettings(t *testing.T, stub *larkStub, ackEmojis string) *fakeEnv {
	t.Helper()
	extra := ""
	if ackEmojis != "" {
		extra = `,"ack_emojis":` + ackEmojis
	}
	return envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+stub.server.URL+`"`+extra+`}`)
}

// TestSendCardLimitFallsBackToPlainText: platform code 11310 (card content
// limit) degrades this one chunk to a plain text message instead of
// dropping the reply; the ids still come back.
func TestSendCardLimitFallsBackToPlainText(t *testing.T) {
	stub := newLarkStub(t, stubOptions{cardCode: 11310, cardMsg: "card content exceeds the limit"})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_chat_1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "long markdown **body**"}},
	})
	if err != nil {
		t.Fatalf("send with card-limit fallback: %v", err)
	}
	if len(ids) != 1 || ids[0] != stubMessageID {
		t.Fatalf("send ids = %v, want the fallback text message id", ids)
	}
	calls := stub.messageCalls()
	if len(calls) != 2 {
		t.Fatalf("message calls = %d, want the card attempt plus the text fallback", len(calls))
	}
	if calls[0].msgType != "interactive" {
		t.Fatalf("first call = %+v, want the interactive card attempt", calls[0])
	}
	if calls[1].msgType != "text" || calls[1].text != "long markdown **body**" {
		t.Fatalf("fallback call = %+v, want the plain text redelivery", calls[1])
	}
}

// TestSendOtherCardErrorSurfaces: any non-zero code other than 11310 keeps
// the failure semantics — the error surfaces and no text fallback fires.
func TestSendOtherCardErrorSurfaces(t *testing.T) {
	stub := newLarkStub(t, stubOptions{cardCode: 230013, cardMsg: "bot ability is off"})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	_, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_chat_1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "230013") {
		t.Fatalf("send error = %v, want code 230013 surfaced", err)
	}
	if got := len(stub.messageCalls()); got != 1 {
		t.Fatalf("message calls = %d, want only the card attempt", got)
	}
}

// TestEditMessagePatchesCardContent: the edit goes out as a card-content
// Patch on the sent message id.
func TestEditMessagePatchesCardContent(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	if err := p.EditMessage(context.Background(), "oc_chat_1", "om_edit_1", plugin.OutboundMessage{
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "edited **body**"}},
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	patches := stub.patchCalls()
	if len(patches) != 1 || !strings.HasSuffix(patches[0].path, "/im/v1/messages/om_edit_1") {
		t.Fatalf("patches = %+v", patches)
	}
	if got := cardMarkdownOf(t, patches[0].content); got != "edited **body**" {
		t.Fatalf("patch content markdown = %q", got)
	}
}

// TestEditMessageRejectsEmptyPayload: no text, no platform call.
func TestEditMessageRejectsEmptyPayload(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	if err := p.EditMessage(context.Background(), "oc_chat_1", "om_1", plugin.OutboundMessage{}); err == nil {
		t.Fatal("empty edit payload must fail")
	}
	if got := len(stub.patchCalls()); got != 0 {
		t.Fatalf("patch calls = %d, want none", got)
	}
}

// TestDeleteMessageRemovesTheSentMessage: the delete names the message id.
func TestDeleteMessageRemovesTheSentMessage(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	if err := p.DeleteMessage(context.Background(), "oc_chat_1", "om_gone_1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deletes := stub.messageDeleteCalls()
	if len(deletes) != 1 || !strings.HasSuffix(deletes[0].path, "/im/v1/messages/om_gone_1") {
		t.Fatalf("deletes = %+v", deletes)
	}
}

// TestPlaceholderSendsCardAndReturnsTheID: the placeholder is the fixed
// copy as a card; its message id is what the Host deletes at the terminal.
func TestPlaceholderSendsCardAndReturnsTheID(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, ""), newFakeWS(nil))

	id, err := p.Placeholder(context.Background(), "oc_chat_1")
	if err != nil {
		t.Fatalf("placeholder: %v", err)
	}
	if id != stubMessageID {
		t.Fatalf("placeholder id = %q, want the stub message id", id)
	}
	calls := stub.messageCalls()
	if len(calls) != 1 || calls[0].msgType != "interactive" || cardMarkdownOf(t, calls[0].content) != placeholderText {
		t.Fatalf("placeholder sends = %+v, want the fixed copy as a card", calls)
	}
}

// TestReactDrawsFromThePoolAndWithdrawsById: the ack reaction uses an emoji
// from the pool (the planted settings name two) and returns the reaction
// id; RemoveReaction deletes exactly that id.
func TestReactDrawsFromThePoolAndWithdrawsById(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, `["THUMBSUP","Pin"]`), newFakeWS(nil))

	reactionID, err := p.React(context.Background(), "oc_chat_1", "om_inbound_1", "")
	if err != nil {
		t.Fatalf("react: %v", err)
	}
	if reactionID != stubReactionID {
		t.Fatalf("reaction id = %q, want the stub id", reactionID)
	}
	creates := stub.reactionCreateCalls()
	if len(creates) != 1 || creates[0].messageID != "om_inbound_1" {
		t.Fatalf("reaction creates = %+v", creates)
	}
	if creates[0].emoji != "THUMBSUP" && creates[0].emoji != "Pin" {
		t.Fatalf("ack emoji = %q, want one of the configured pool", creates[0].emoji)
	}

	if err := p.RemoveReaction(context.Background(), "oc_chat_1", "om_inbound_1", reactionID); err != nil {
		t.Fatalf("remove reaction: %v", err)
	}
	deletes := stub.reactionDeleteCalls()
	if len(deletes) != 1 || deletes[0].messageID != "om_inbound_1" || deletes[0].reactionID != stubReactionID {
		t.Fatalf("reaction deletes = %+v", deletes)
	}
}

// TestReactDisabledWhenPoolEmpty: an explicit empty ack_emojis list
// disables the ack with a reported error, touching no endpoint.
func TestReactDisabledWhenPoolEmpty(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	p, _ := startWithFake(t, feishuSettings(t, stub, `[]`), newFakeWS(nil))

	if _, err := p.React(context.Background(), "oc_chat_1", "om_inbound_1", ""); err == nil {
		t.Fatal("react with an empty pool must fail")
	}
	if got := len(stub.reactionCreateCalls()); got != 0 {
		t.Fatalf("reaction creates = %d, want none", got)
	}
}

// TestInteractionFacesFailClosedWhenNotStarted: the optional faces are
// useless before Start and must say so.
func TestInteractionFacesFailClosedWhenNotStarted(t *testing.T) {
	p := newAdapter()
	ctx := context.Background()
	if err := p.EditMessage(ctx, "c", "m", plugin.OutboundMessage{Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("edit before start must fail")
	}
	if err := p.DeleteMessage(ctx, "c", "m"); err == nil {
		t.Fatal("delete before start must fail")
	}
	if _, err := p.Placeholder(ctx, "c"); err == nil {
		t.Fatal("placeholder before start must fail")
	}
	if _, err := p.React(ctx, "c", "m", ""); err == nil {
		t.Fatal("react before start must fail")
	}
	if err := p.RemoveReaction(ctx, "c", "m", "r"); err == nil {
		t.Fatal("remove reaction before start must fail")
	}
}

// TestAckEmojiPool: nil decodes to the default pool, an explicit empty
// list disables, empty entries are dropped, and entries are trimmed.
func TestAckEmojiPool(t *testing.T) {
	if got := (Settings{}).ackEmojiPool(); !reflect.DeepEqual(got, []string{"THUMBSUP"}) {
		t.Fatalf("default pool = %v, want [THUMBSUP]", got)
	}
	if got := (Settings{AckEmojis: []string{}}).ackEmojiPool(); len(got) != 0 {
		t.Fatalf("explicit empty pool = %v, want empty", got)
	}
	got := (Settings{AckEmojis: []string{"", " Pin ", "DONE"}}).ackEmojiPool()
	if !reflect.DeepEqual(got, []string{"Pin", "DONE"}) {
		t.Fatalf("pool = %v, want [Pin DONE]", got)
	}
}
