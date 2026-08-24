# Eino schema.Message & templates (v0.9.13)

Source: `schema/message.go`, `schema/tool.go` in the local module cache.

## Role constants

```go
schema.Assistant // "assistant"
schema.User      // "user"
schema.System    // "system"
schema.Tool      // "tool"
```

## Message struct (key fields)

```go
type Message struct {
    Role        RoleType `json:"role"`
    Content     string   `json:"content"`     // user text input / model text output

    // Deprecated: use UserInputMultiContent / AssistantGenMultiContent instead.
    MultiContent []ChatMessagePart `json:"multi_content,omitempty"`

    // Multimodal user input (text + image/audio/video/file parts).
    UserInputMultiContent []MessageInputPart `json:"user_input_multi_content,omitempty"`
    // Multimodal model output.
    AssistantGenMultiContent []MessageOutputPart `json:"assistant_output_multi_content,omitempty"`

    Name string `json:"name,omitempty"`

    // assistant only
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
    // tool only
    ToolCallID string `json:"tool_call_id,omitempty"`
    ToolName   string `json:"tool_name,omitempty"`

    ResponseMeta *ResponseMeta `json:"response_meta,omitempty"` // FinishReason + Usage
    ReasoningContent string `json:"reasoning_content,omitempty"`
    Extra map[string]any `json:"extra,omitempty"`
}
```

`ResponseMeta.Usage` holds `PromptTokens / CompletionTokens / TotalTokens` and `CompletionTokensDetails.ReasoningTokens`.

## ToolCall (assistant message)

```go
type ToolCall struct {
    Index    *int         // stream chunk grouping only
    ID       string
    Type     string       // usually "function"
    Function FunctionCall // Name + Arguments (JSON string)
    Extra    map[string]any
}
```

## Constructors

```go
schema.SystemMessage("you are an eino helper")                 // Role: system
schema.UserMessage("what is eino?")                            // Role: user
schema.AssistantMessage("eino is a framework", nil)            // Role: assistant, optional []ToolCall
schema.ToolMessage(content, toolCallID, schema.WithToolName(name)) // Role: tool
// raw form for anything else:
&schema.Message{Role: schema.Assistant, Content: "..."}
```

## ToolInfo & parameter schema

```go
info := &schema.ToolInfo{
    Name: "get_weather",
    Desc: "Get current weather for a city. Use when the user asks about weather.",
}
info.ParamsOneOf = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
    "city": {Type: schema.String, Desc: "city name", Required: true},
})
// or full JSON Schema:
info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(jsonschema)
// nil ParamsOneOf => tool takes no arguments
```

`ParameterInfo{Type DataType, ElemInfo, SubParams, Desc, Enum []string, Required bool}`; `DataType` constants include `schema.String`, `schema.Number`, `schema.Boolean`, `schema.Integer`, `schema.Array`, `schema.Object`.

`ToolChoice` constants (schema/tool.go), passed via `model.WithToolChoice`:
`schema.ToolChoiceForbidden` (no tools), `schema.ToolChoiceAllowed` (model decides), `schema.ToolChoiceForced` (must call at least one).

## Message templates (prompt layer)

`Message` and `MessagesPlaceholder` implement `MessagesTemplate`:

```go
type MessagesTemplate interface {
    Format(ctx context.Context, vs map[string]any, formatType FormatType) ([]*Message, error)
}
```

```go
chatTpl := prompt.FromMessages(
    schema.FString,
    schema.SystemMessage("you are an eino helper"),
    schema.MessagesPlaceholder("history", false), // pulls []*Message from params["history"]
)
msgs, err := chatTpl.Format(ctx, map[string]any{
    "history": history,          // []*schema.Message
    "query":   "how to use eino?",
})
```

`FormatType`: `schema.FString` (Python-style, via pyfmt), `schema.GoTemplate`, `schema.Jinja2`.
If the placeholder key is missing and `optional` is false, `Format` errors; if optional, it yields `[]*Message{}`.
`prompt.FromMessages(formatType, templates...)` returns a `*prompt.DefaultChatTemplate` implementing `prompt.ChatTemplate` (usable in Chain/Graph).

## Multimodal parts

User input:

```go
&schema.Message{
    Role: schema.User,
    UserInputMultiContent: []schema.MessageInputPart{
        {Type: schema.ChatMessagePartTypeText, Text: "What is in this image?"},
        {Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
            MessagePartCommon: schema.MessagePartCommon{URL: toPtr("https://.../cat.jpg")},
            Detail: schema.ImageURLDetailHigh,
        }},
    },
}
```

Model output parts arrive in `AssistantGenMultiContent` (`ChatMessagePartTypeText` / `Image` / `Audio` / `Video` / `Reasoning`).
