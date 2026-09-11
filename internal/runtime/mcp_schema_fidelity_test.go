package runtime

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPHostListToolsPreservesRawJSONSchemaKeywords(t *testing.T) {
	const schema = `{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"$defs":{"positive":{"type":"integer","minimum":1}},
		"type":"object",
		"properties":{
			"mode":{"type":"string","enum":["fast","safe"]},
			"items":{"type":"array","minItems":1,"items":{"$ref":"#/$defs/positive"}}
		},
		"required":["mode","items"],
		"additionalProperties":false,
		"minProperties":2,
		"unevaluatedProperties":false
	}`
	var rawCall json.RawMessage
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: 202, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			var value any
			if err := json.Unmarshal([]byte(schema), &value); err != nil {
				t.Fatal(err)
			}
			return testMCPResponse{Result: map[string]any{"tools": []any{map[string]any{
				"name": "constrained", "description": "raw schema", "inputSchema": value,
			}}}}
		case "tools/call":
			rawCall = append(json.RawMessage(nil), request.RawParams...)
			return testMCPResponse{Result: map[string]any{"content": []any{map[string]any{"type": "text", "text": "ok"}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "raw-schema")
	tools, err := backend.hostListTools(context.Background(), "raw-schema")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %#v", tools)
	}
	var got, want any
	if err := json.Unmarshal(tools[0].schema, &got); err != nil {
		t.Fatalf("projected schema is invalid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(schema), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("raw schema changed\n got: %s\nwant: %s", tools[0].schema, schema)
	}
	listed, err := backend.ListTools(context.Background(), "", "raw-schema")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 1 || !reflect.DeepEqual(json.RawMessage(listed.Tools[0].InputSchema), tools[0].schema) {
		t.Fatalf("control-plane schema changed: %#v", listed.Tools)
	}
	if _, err := backend.hostCallToolRaw(context.Background(), "raw-schema", "constrained", json.RawMessage(`{"value":9007199254740993}`)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawCall), "9007199254740993") {
		t.Fatalf("raw MCP arguments lost integer precision: %s", rawCall)
	}
}

func TestMCPListToolsDoesNotReuseSchemaWhenResponseOmitsIt(t *testing.T) {
	const firstSchema = `{"type":"object","properties":{"value":{"type":"integer","minimum":1}}}`
	var lists int
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: 202, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			lists++
			if lists == 1 {
				var value any
				if err := json.Unmarshal([]byte(firstSchema), &value); err != nil {
					t.Fatal(err)
				}
				return testMCPResponse{Result: map[string]any{"tools": []any{map[string]any{
					"name": "echo", "description": "first", "inputSchema": value,
				}}}}
			}
			// Deliberately omit inputSchema. The second response must not
			// inherit the first request's schema from a server-scoped cache.
			return testMCPResponse{Result: map[string]any{"tools": []any{map[string]any{
				"name": "echo", "description": "omitted",
			}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "scoped")
	first, err := backend.hostListTools(context.Background(), "scoped")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || string(first[0].schema) != firstSchema {
		var got, want any
		if err := json.Unmarshal(first[0].schema, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(firstSchema), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("first schema = %#v, want %s", first, firstSchema)
		}
	}
	second, err := backend.hostListTools(context.Background(), "scoped")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || len(second[0].schema) != 0 {
		t.Fatalf("omitted schema reused prior response: %#v", second)
	}
}

func TestMCPRawSchemaCapturesAreRequestScoped(t *testing.T) {
	capturing := newRawMCPToolSchemas()
	firstCtx, first := withRawMCPToolSchemaCapture(context.Background())
	secondCtx, second := withRawMCPToolSchemaCapture(context.Background())
	transport := &capturingMCPTransport{rawSchemas: capturing}
	response := func(schema string) *mcptransport.JSONRPCResponse {
		return &mcptransport.JSONRPCResponse{Result: json.RawMessage(`{"tools":[{"name":"echo","inputSchema":` + schema + `}]}`)}
	}
	_ = transport
	_ = mcptransport.JSONRPCRequest{Method: string(mcp.MethodToolsList)}
	// Feed the captures through the explicit capture seam; each request must
	// own its response map even when the remote name is identical.
	capturing.capture(firstCtx, response(`{"type":"string"}`))
	capturing.capture(secondCtx, response(`{"type":"integer"}`))
	if got, ok := first.get("echo"); !ok || string(got) != `{"type":"string"}` {
		t.Fatalf("first request capture = %s, %v", got, ok)
	}
	if got, ok := second.get("echo"); !ok || string(got) != `{"type":"integer"}` {
		t.Fatalf("second request capture = %s, %v", got, ok)
	}
}
