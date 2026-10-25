package runtime

import (
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

// The request shape is the content-free structural description of the exact
// message list handed to the model. One traversal feeds the v5 envelope, the
// pre-send validator and the retry fingerprint, so the audit cannot describe a
// different input than the one that was checked or replayed.

// modelShapeBlockCap bounds the ordered block lists one v5 request publishes.
// Counts stay complete; only the lists stop growing.
const modelShapeBlockCap = 256

// modelShapeParamCap bounds one tool declaration's published parameters.
const modelShapeParamCap = 64

// modelShapeStringCap bounds every identifier copied into the envelope. Bytes
// are a conservative proxy for the schema's character bound.
const modelShapeStringCap = 128

// modelShape is the traversal result shared by the envelope, validator and
// fingerprint. A nil message is represented as an empty row: the validator
// rejects it separately.
type modelShape struct {
	Messages      []modelMessageShape
	CallIDs       []string
	ResultCallIDs []string
	Counts        payloadRequestCounts
	PreambleBytes int
	Truncated     bool
}

type modelMessageShape struct {
	Role       string
	ByteLen    int
	ToolName   string
	ToolCallID string
	Blocks     []payloadRequestBlock
}

func describeModelShape(msgs []*schema.AgenticMessage) modelShape {
	var out modelShape
	stored := 0
	for i, msg := range msgs {
		if msg == nil {
			continue
		}
		row := modelMessageShape{
			Role:    agenticRoleName(msg),
			ByteLen: len(modelRequestText(msg)),
		}
		if i == 0 && msg.Role == schema.AgenticRoleTypeSystem {
			out.PreambleBytes = row.ByteLen
		}
		out.Counts.Messages++
		switch row.Role {
		case "system":
			out.Counts.System++
		case "user":
			out.Counts.User++
		case "assistant":
			out.Counts.Assistant++
		case "tool":
			out.Counts.Tool++
		}
		for _, block := range msg.ContentBlocks {
			if block == nil {
				continue
			}
			out.Counts.Blocks++
			shape := describeRequestBlock(block)
			switch shape.Kind {
			case string(schema.ContentBlockTypeFunctionToolCall):
				out.Counts.ToolCalls++
				row.ToolName = shape.Name
				row.ToolCallID = shape.CallID
				if stored < modelShapeBlockCap {
					out.CallIDs = append(out.CallIDs, shape.CallID)
				}
			case string(schema.ContentBlockTypeFunctionToolResult):
				out.Counts.ToolResults++
				row.ToolCallID = shape.CallID
				if stored < modelShapeBlockCap {
					out.ResultCallIDs = append(out.ResultCallIDs, shape.CallID)
				}
			}
			if stored < modelShapeBlockCap {
				row.Blocks = append(row.Blocks, shape)
				stored++
			} else {
				out.Truncated = true
			}
		}
		out.Messages = append(out.Messages, row)
	}
	return out
}

func describeRequestBlock(block *schema.ContentBlock) payloadRequestBlock {
	out := payloadRequestBlock{Kind: contentBlockKind(block)}
	switch {
	case block.Reasoning != nil:
		out.ByteLen = len(block.Reasoning.Text)
	case block.UserInputText != nil:
		out.ByteLen = len(block.UserInputText.Text)
	case block.AssistantGenText != nil:
		out.ByteLen = len(block.AssistantGenText.Text)
	case block.FunctionToolCall != nil:
		out.CallID = clampShapeString(block.FunctionToolCall.CallID)
		out.Name = clampShapeString(block.FunctionToolCall.Name)
	case block.FunctionToolResult != nil:
		result := block.FunctionToolResult
		out.CallID = clampShapeString(result.CallID)
		out.Name = clampShapeString(result.Name)
		out.ByteLen = len(agenticToolResultText(result))
		for _, part := range result.Content {
			if part == nil {
				continue
			}
			out.ContentKinds = append(out.ContentKinds, string(part.Type))
		}
	}
	return out
}

// contentBlockKind prefers the declared type and only infers it for blocks
// built without the typed constructor.
func contentBlockKind(block *schema.ContentBlock) string {
	if block.Type != "" {
		return string(block.Type)
	}
	switch {
	case block.Reasoning != nil:
		return string(schema.ContentBlockTypeReasoning)
	case block.UserInputText != nil:
		return string(schema.ContentBlockTypeUserInputText)
	case block.UserInputImage != nil:
		return string(schema.ContentBlockTypeUserInputImage)
	case block.UserInputAudio != nil:
		return string(schema.ContentBlockTypeUserInputAudio)
	case block.UserInputVideo != nil:
		return string(schema.ContentBlockTypeUserInputVideo)
	case block.UserInputFile != nil:
		return string(schema.ContentBlockTypeUserInputFile)
	case block.AssistantGenText != nil:
		return string(schema.ContentBlockTypeAssistantGenText)
	case block.AssistantGenImage != nil:
		return string(schema.ContentBlockTypeAssistantGenImage)
	case block.AssistantGenAudio != nil:
		return string(schema.ContentBlockTypeAssistantGenAudio)
	case block.AssistantGenVideo != nil:
		return string(schema.ContentBlockTypeAssistantGenVideo)
	case block.FunctionToolCall != nil:
		return string(schema.ContentBlockTypeFunctionToolCall)
	case block.FunctionToolResult != nil:
		return string(schema.ContentBlockTypeFunctionToolResult)
	case block.ToolSearchFunctionToolResult != nil:
		return string(schema.ContentBlockTypeToolSearchResult)
	case block.ServerToolCall != nil:
		return string(schema.ContentBlockTypeServerToolCall)
	case block.ServerToolResult != nil:
		return string(schema.ContentBlockTypeServerToolResult)
	case block.MCPToolCall != nil:
		return string(schema.ContentBlockTypeMCPToolCall)
	case block.MCPToolResult != nil:
		return string(schema.ContentBlockTypeMCPToolResult)
	case block.MCPListToolsResult != nil:
		return string(schema.ContentBlockTypeMCPListToolsResult)
	case block.MCPToolApprovalRequest != nil:
		return string(schema.ContentBlockTypeMCPToolApprovalRequest)
	case block.MCPToolApprovalResponse != nil:
		return string(schema.ContentBlockTypeMCPToolApprovalResponse)
	default:
		return "unknown"
	}
}

// describeModelRequestV5 builds the v5 envelope: the v4 row metadata plus the
// ordered structural shape and the tool declarations bound for this call.
func describeModelRequestV5(msgs []*schema.AgenticMessage, tools []*schema.ToolInfo) payloadModelRequestV5 {
	shape := describeModelShape(msgs)
	return payloadModelRequestV5{
		payloadModelRequestV4: payloadModelRequestV4{
			payloadModelRequest: payloadRequestFromShape(shape, toolInfoNames(tools)),
		},
		CallIDs:          shape.CallIDs,
		ResultCallIDs:    shape.ResultCallIDs,
		ToolDeclarations: describeToolDeclarations(tools),
		Counts:           shape.Counts,
		ShapeTruncated:   shape.Truncated,
	}
}

// describeToolDeclarations publishes names, ordered parameter names, scalar
// types and required flags — never schema bytes.
func describeToolDeclarations(infos []*schema.ToolInfo) []payloadToolDeclaration {
	out := make([]payloadToolDeclaration, 0, len(infos))
	for _, info := range infos {
		if info == nil || info.Name == "" {
			continue
		}
		decl := payloadToolDeclaration{Name: clampShapeString(info.Name)}
		decl.Parameters = describeToolParams(info.ParamsOneOf)
		out = append(out, decl)
	}
	return out
}

func describeToolParams(params *schema.ParamsOneOf) []payloadToolParam {
	if params == nil {
		return nil
	}
	document, err := params.ToJSONSchema()
	if err != nil || document == nil {
		return nil
	}
	return describeSchemaParams(document)
}

func describeSchemaParams(document *jsonschema.Schema) []payloadToolParam {
	if document == nil || document.Properties == nil {
		return nil
	}
	required := make(map[string]struct{}, len(document.Required))
	for _, name := range document.Required {
		required[name] = struct{}{}
	}
	out := make([]payloadToolParam, 0, document.Properties.Len())
	for pair := document.Properties.Oldest(); pair != nil; pair = pair.Next() {
		if len(out) >= modelShapeParamCap {
			break
		}
		if pair.Key == "" {
			continue
		}
		param := payloadToolParam{Name: clampShapeString(pair.Key)}
		if pair.Value != nil {
			param.Type = pair.Value.Type
		}
		if _, ok := required[pair.Key]; ok {
			param.Required = true
		}
		out = append(out, param)
	}
	return out
}

// clampShapeString bounds identifiers without publishing content and without
// the event-payload warning that content clamping emits.
func clampShapeString(value string) string {
	if len(value) <= modelShapeStringCap {
		return value
	}
	return strings.ToValidUTF8(value[:modelShapeStringCap], "")
}
