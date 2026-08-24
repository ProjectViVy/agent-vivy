package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// EchoInfoName is the registered name of the read-only auto-execute tool
// (D-012).
const EchoInfoName = "echo_info"

// echoTextLimit bounds the echoed payload (NFR: bounded).
const echoTextLimit = 4096

// echoInfo returns the caller-provided text verbatim. It is read-only: no
// storage, no network, no side effects, so it auto-executes without
// approval while still emitting tool.started/tool.finished events.
type echoInfo struct{}

// NewEchoInfo returns the read-only echo tool.
func NewEchoInfo() Tool { return echoInfo{} }

func (echoInfo) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        EchoInfoName,
		Description: "Echoes the provided text back verbatim. Read-only, no approval needed.",
		Readonly:    true,
		Keywords:    []string{"echo", "repeat"},
		Params: map[string]domain.ToolParam{
			"text": {Desc: "The text to echo back verbatim.", Required: true},
		},
	}
}

type echoArgs struct {
	Text string `json:"text"`
}

func (echoInfo) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	var parsed echoArgs
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return "", &ArgError{Field: "args", Reason: fmt.Sprintf("must be a JSON object {\"text\": string}: %v", err)}
	}
	if parsed.Text == "" {
		return "", &ArgError{Field: "text", Reason: "must be a non-empty string"}
	}
	if len(parsed.Text) > echoTextLimit {
		return "", &ArgError{Field: "text", Reason: fmt.Sprintf("exceeds %d bytes", echoTextLimit)}
	}
	return parsed.Text, nil
}
