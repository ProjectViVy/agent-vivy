package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestD14ShellPreviewReplayUsesCurrentRedaction(t *testing.T) {
	for _, command := range []string{"bash script [redacted bytes=0]", "bash script [redacted bytes=123]"} {
		args, _ := json.Marshal(map[string]string{"command": command})
		result := toMessageResult(domain.Message{ToolName: tools.BashName, ToolArgs: args}, false)
		if result.ToolPreview != command {
			t.Fatalf("current redacted preview lost on replay: %+v", result)
		}
	}
	for _, command := range []string{"echo private-command", "bash script [redacted bytes=1] trailing", "bash script [redacted bytes=-1]"} {
		args, _ := json.Marshal(map[string]string{"command": command})
		if result := toMessageResult(domain.Message{ToolName: tools.BashName, ToolArgs: args}, false); result.ToolPreview != "" {
			t.Fatalf("untrusted bash argument exposed: %+v", result)
		}
	}
}

func TestD14MCPCommandNameKeepsExactPromptIdentity(t *testing.T) {
	seen := make(map[string]bool)
	pairs := [][2]string{{"docs server", "review/change"}, {"docs-server", "review-change"}, {"Docs server", "review/change"}, {"docs server", "Review/change"}, {strings.Repeat("s", 256), strings.Repeat("p", 256)}}
	for _, pair := range pairs {
		name := mcpPromptCommandName(pair[0], pair[1])
		parts := strings.Split(strings.TrimPrefix(name, "mcp-"), "-")
		if len(parts) != 2 {
			t.Fatalf("command identity is not reversible: %q", name)
		}
		server, serverErr := hex.DecodeString(parts[0])
		prompt, promptErr := hex.DecodeString(parts[1])
		if serverErr != nil || promptErr != nil || string(server) != pair[0] || string(prompt) != pair[1] {
			t.Fatalf("command lost exact server/prompt identity: %q", name)
		}
		if name != strings.ToLower(name) || seen[name] {
			t.Fatalf("lowercase command identity collision: %q", name)
		}
		seen[name] = true
	}
	stub := &mcpCatalogStub{prompt: tools.MCPGetPromptResponse{Text: "expanded", Untrusted: true}}
	for _, pair := range pairs {
		stub.prompts.Prompts = append(stub.prompts.Prompts, tools.MCPPrompt{Server: pair[0], Name: pair[1]})
	}
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) { deps.MCP = stub })
	listed, rpcErr := callControl(t, env.handler, "commands/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(listed)
	var result struct {
		Commands []dynamicCommandResult `json:"commands"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Commands) != len(pairs) {
		t.Fatalf("valid prompt identity disappeared from commands/list: %d", len(result.Commands))
	}
	for _, pair := range pairs {
		id := encodeMCPPromptCommandID(pair[0], pair[1])
		if _, rpcErr := callControl(t, env.handler, "commands/expand", map[string]any{"id": id}); rpcErr != nil {
			t.Fatal(rpcErr)
		}
		if stub.promptRequest.Server != pair[0] || stub.promptRequest.Name != pair[1] {
			t.Fatal("command expansion changed the original server/prompt identity")
		}
	}
}
