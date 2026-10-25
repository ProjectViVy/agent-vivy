package runtime

import (
	"fmt"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestDescribeModelRequestV5KeepsEveryToolIdentifier pins the v4 blind spot:
// only the first id of a multi-call or multi-result message used to reach the
// journal. The v5 shape must publish every id in input order and the per-block
// identifiers that explain them.
func TestDescribeModelRequestV5KeepsEveryToolIdentifier(t *testing.T) {
	msgs := []*schema.AgenticMessage{
		testSystem("sys"),
		testUser("go"),
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.Reasoning{Text: "think"}),
				schema.NewContentBlock(&schema.AssistantGenText{Text: "calling"}),
				schema.NewContentBlock(testCall("call-a", "echo_info", `{"text":"a"}`)),
				schema.NewContentBlock(testCall("call-b", "read_file", `{"path":"b"}`)),
			},
		},
		agenticUserBlocks(append(
			testToolResult("call-a", "echo_info", "a-ok").ContentBlocks,
			testToolResult("call-b", "read_file", "b-ok").ContentBlocks...,
		)),
	}
	req := describeModelRequestV5(msgs, nil)

	if got, want := req.CallIDs, []string{"call-a", "call-b"}; !equalStrings(got, want) {
		t.Fatalf("call ids = %v, want %v", got, want)
	}
	if got, want := req.ResultCallIDs, []string{"call-a", "call-b"}; !equalStrings(got, want) {
		t.Fatalf("result ids = %v, want %v", got, want)
	}
	if req.Counts != (payloadRequestCounts{Messages: 4, Blocks: 8, ToolCalls: 2, ToolResults: 2, System: 1, User: 1, Assistant: 1, Tool: 1}) {
		t.Fatalf("counts = %+v", req.Counts)
	}

	assistant := req.Messages[2]
	if assistant.Role != "assistant" {
		t.Fatalf("assistant role = %q", assistant.Role)
	}
	if len(assistant.Blocks) != 4 {
		t.Fatalf("assistant blocks = %+v, want 4", assistant.Blocks)
	}
	reasoning := assistant.Blocks[0]
	if reasoning.Kind != string(schema.ContentBlockTypeReasoning) || reasoning.ByteLen != len("think") {
		t.Fatalf("reasoning block = %+v", reasoning)
	}
	// agenticMessageText excludes reasoning, so the row byte length covers only
	// the assistant text; the block row is where reasoning bytes stay visible.
	if assistant.ByteLen != len("calling") {
		t.Fatalf("assistant byte_len = %d, want %d", assistant.ByteLen, len("calling"))
	}
	if got := assistant.Blocks[2]; got.Kind != string(schema.ContentBlockTypeFunctionToolCall) ||
		got.CallID != "call-a" || got.Name != "echo_info" {
		t.Fatalf("first call block = %+v", got)
	}
	if got := assistant.Blocks[3]; got.CallID != "call-b" || got.Name != "read_file" {
		t.Fatalf("second call block = %+v", got)
	}

	results := req.Messages[3]
	if len(results.Blocks) != 2 {
		t.Fatalf("result blocks = %+v, want 2", results.Blocks)
	}
	for i, want := range []string{"call-a", "call-b"} {
		block := results.Blocks[i]
		if block.Kind != string(schema.ContentBlockTypeFunctionToolResult) || block.CallID != want {
			t.Fatalf("result block %d = %+v, want call %q", i, block, want)
		}
		if !equalStrings(block.ContentKinds, []string{string(schema.FunctionToolResultContentBlockTypeText)}) {
			t.Fatalf("result block %d content kinds = %v", i, block.ContentKinds)
		}
	}
}

// TestDescribeModelRequestV5KeepsAdjacentSystemRows reproduces the failing
// session's input shape: two system rows, no tool turns. The envelope must
// publish both rows and report the first one as the preamble.
func TestDescribeModelRequestV5KeepsAdjacentSystemRows(t *testing.T) {
	msgs := []*schema.AgenticMessage{
		testSystem("preamble"),
		testSystem("second"),
		testUser("question"),
		testAssistant("answer"),
		testUser("follow-up"),
	}
	req := describeModelRequestV5(msgs, nil)

	if req.Counts != (payloadRequestCounts{Messages: 5, Blocks: 5, System: 2, User: 2, Assistant: 1}) {
		t.Fatalf("counts = %+v", req.Counts)
	}
	if req.PreambleBytes != len("preamble") {
		t.Fatalf("preamble bytes = %d, want %d", req.PreambleBytes, len("preamble"))
	}
	wantRoles := []string{"system", "system", "user", "assistant", "user"}
	for i, want := range wantRoles {
		if req.Messages[i].Role != want {
			t.Fatalf("message %d role = %q, want %q", i, req.Messages[i].Role, want)
		}
	}
	if req.ShapeTruncated {
		t.Fatal("short input reported as truncated")
	}
}

// TestDescribeModelRequestV5PublishesToolDeclarations keeps the bound tool
// contract auditable without publishing schema bytes.
func TestDescribeModelRequestV5PublishesToolDeclarations(t *testing.T) {
	tools := []*schema.ToolInfo{
		{
			Name: "echo_info",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"text":  {Type: schema.String, Required: true},
				"limit": {Type: schema.Integer},
			}),
		},
		{Name: "noargs"},
	}
	req := describeModelRequestV5(nil, tools)

	if len(req.ToolDeclarations) != 2 {
		t.Fatalf("declarations = %+v, want 2", req.ToolDeclarations)
	}
	echo := req.ToolDeclarations[0]
	if echo.Name != "echo_info" || len(echo.Parameters) != 2 {
		t.Fatalf("echo declaration = %+v", echo)
	}
	params := map[string]payloadToolParam{}
	for _, param := range echo.Parameters {
		params[param.Name] = param
	}
	if got := params["text"]; got.Type != string(schema.String) || !got.Required {
		t.Fatalf("text param = %+v", got)
	}
	if got := params["limit"]; got.Type != string(schema.Integer) || got.Required {
		t.Fatalf("limit param = %+v", got)
	}
	if got := req.ToolDeclarations[1]; got.Name != "noargs" || len(got.Parameters) != 0 {
		t.Fatalf("noargs declaration = %+v", got)
	}
}

// TestDescribeModelShapeCapsOrderedLists: the ordered lists stop at the cap,
// but the counts stay complete so a truncated shape is still measurable.
func TestDescribeModelShapeCapsOrderedLists(t *testing.T) {
	blocks := make([]*schema.ContentBlock, 0, modelShapeBlockCap+44)
	for i := 0; i < modelShapeBlockCap+44; i++ {
		blocks = append(blocks, schema.NewContentBlock(
			testCall(fmt.Sprintf("call-%03d", i), "echo_info", `{}`),
		))
	}
	msgs := []*schema.AgenticMessage{{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: blocks}}

	shape := describeModelShape(msgs)
	if !shape.Truncated {
		t.Fatal("shape above the block cap did not report truncation")
	}
	if len(shape.CallIDs) != modelShapeBlockCap {
		t.Fatalf("stored call ids = %d, want cap %d", len(shape.CallIDs), modelShapeBlockCap)
	}
	if len(shape.Messages) != 1 || len(shape.Messages[0].Blocks) != modelShapeBlockCap {
		t.Fatalf("stored blocks = %+v", shape.Messages)
	}
	if shape.Counts.Blocks != modelShapeBlockCap+44 || shape.Counts.ToolCalls != modelShapeBlockCap+44 {
		t.Fatalf("complete counts = %+v", shape.Counts)
	}
	if shape.CallIDs[0] != "call-000" || shape.CallIDs[len(shape.CallIDs)-1] != fmt.Sprintf("call-%03d", modelShapeBlockCap-1) {
		t.Fatalf("call id order = %q ... %q", shape.CallIDs[0], shape.CallIDs[len(shape.CallIDs)-1])
	}
}

// TestDescribeModelRequestV5ClampsIdentifiers bounds every identifier copied
// into the envelope while leaving content untouched.
func TestDescribeModelRequestV5ClampsIdentifiers(t *testing.T) {
	longID := ""
	for len(longID) < modelShapeStringCap*2 {
		longID += "x"
	}
	msgs := []*schema.AgenticMessage{testToolCalls(testCall(longID, longID, `{}`))}
	req := describeModelRequestV5(msgs, nil)

	if len(req.CallIDs) != 1 || len(req.CallIDs[0]) != modelShapeStringCap {
		t.Fatalf("clamped call id length = %d, want %d", len(req.CallIDs[0]), modelShapeStringCap)
	}
	block := req.Messages[0].Blocks[0]
	if len(block.CallID) != modelShapeStringCap || len(block.Name) != modelShapeStringCap {
		t.Fatalf("clamped block = %+v", block)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
