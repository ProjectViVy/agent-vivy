package mcphost

import (
	"context"
	"strconv"

	statusport "agent-vivy/sdk/port/status"
)

// StatusSource is a read-only std/status-source@v1 projection. It reads only
// MCPHost's in-memory instance records and never opens, probes, revives, or
// reconfigures a remote MCP session.
type StatusSource struct {
	host *Host
}

func NewStatusSource(host *Host) *StatusSource { return &StatusSource{host: host} }
func (*StatusSource) ID() string               { return "mcp" }

func (source *StatusSource) Status(context.Context, statusport.Request) (statusport.Snapshot, error) {
	if source == nil || source.host == nil {
		return statusport.NewSnapshot("", "", nil), nil
	}
	statuses := source.host.Status()
	items := make([]statusport.Item, 0, len(statuses))
	for _, status := range statuses {
		fields := map[string]string{
			"tool_count":            strconv.Itoa(status.ToolCount),
			"consecutive_failures": strconv.Itoa(status.ConsecutiveFailures),
			"circuit_open":          strconv.FormatBool(status.CircuitOpen),
			"resource_bridge":       strconv.FormatBool(status.ResourceBridge),
		}
		if status.DeferredReason != "" {
			fields["deferred_reason"] = status.DeferredReason
		}
		items = append(items, statusport.Item{ID: status.ID, State: string(status.State), Fields: fields})
	}
	return statusport.NewSnapshot("mcp-host-v1", "", items), nil
}

var _ statusport.Provider = (*StatusSource)(nil)
