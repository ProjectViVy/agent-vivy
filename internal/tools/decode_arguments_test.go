package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// A decode failure in a tool's own argument handling is the model's mistake,
// not an infrastructure fault: every tool that parses its arguments itself must
// report *ArgError so the runtime's §5 soft conversion (NUDGE-DESIGN) keeps the
// run alive and tells the model what to fix. The adapter's ValidateArgs catches
// most malformed calls earlier; these checks keep the tool layer consistent for
// every other caller and for shapes the declared params do not cover.
func TestToolArgumentDecodeFailuresAreArgumentErrors(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		args json.RawMessage
		call func(json.RawMessage) error
	}{
		{"bash", json.RawMessage(`{"command":123}`), func(args json.RawMessage) error {
			_, _, err := decodeBashRequest(args)
			return err
		}},
		{"command", json.RawMessage(`{"command":123}`), func(args json.RawMessage) error {
			_, err := decodeCommandRequest(args)
			return err
		}},
		{"download", json.RawMessage(`{"url":123}`), func(args json.RawMessage) error {
			_, err := decodeDownloadRequest(args)
			return err
		}},
		{"http_request", json.RawMessage(`{"url":123}`), func(args json.RawMessage) error {
			_, err := (&httpRequestTool{}).InvokableRun(ctx, args)
			return err
		}},
		{"jobs", json.RawMessage(`{"job_id":123}`), func(args json.RawMessage) error {
			_, err := (&jobsTool{}).InvokableRun(ctx, args)
			return err
		}},
		{"mcp_list_tools", json.RawMessage(`{"server":123}`), func(args json.RawMessage) error {
			_, err := (&mcpListToolsTool{}).InvokableRun(ctx, args)
			return err
		}},
		{"network_search", json.RawMessage(`{"query":123}`), func(args json.RawMessage) error {
			_, err := (&networkSearchTool{}).InvokableRun(ctx, args)
			return err
		}},
		{"sequential_thinking", json.RawMessage(`{"thought":123}`), func(args json.RawMessage) error {
			_, err := (&sequentialThinkingTool{}).InvokableRun(ctx, args)
			return err
		}},
		{"web_fetch", json.RawMessage(`{"url":123}`), func(args json.RawMessage) error {
			_, err := (&webFetchTool{}).InvokableRun(ctx, args)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(tc.args)
			var argErr *ArgError
			if !errors.As(err, &argErr) || argErr.Field != "args" {
				t.Fatalf("error = %v (%T), want *ArgError on field %q", err, err, "args")
			}
			if !strings.Contains(argErr.Error(), "invalid arguments") {
				t.Fatalf("diagnostic = %q, want the decode cause preserved", argErr.Error())
			}
		})
	}
	// Malformed JSON must reach the same typed refusal for every tool.
	for _, tc := range cases {
		t.Run(tc.name+"/malformed-json", func(t *testing.T) {
			err := tc.call(json.RawMessage(`{not json`))
			var argErr *ArgError
			if !errors.As(err, &argErr) || argErr.Field != "args" {
				t.Fatalf("error = %v (%T), want *ArgError on field %q", err, err, "args")
			}
		})
	}
}

// A job id that names no job of the run is an argument mistake, not a backend
// failure: the model must be able to list the run's jobs and retry.
func TestJobsToolUnknownJobIsArgumentError(t *testing.T) {
	tool := &jobsTool{name: JobOutputName, ops: jobReadStub{}}
	_, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"job_id":"job-missing"}`))
	var argErr *ArgError
	if !errors.As(err, &argErr) || argErr.Field != "job_id" {
		t.Fatalf("error = %v (%T), want *ArgError on field %q", err, err, "job_id")
	}
}

// jobReadStub reports no remembered jobs.
type jobReadStub struct{}

func (jobReadStub) JobRead(string) (JobReadResult, bool)  { return JobReadResult{}, false }
func (jobReadStub) JobKill(string) (JobKillResult, error) { return JobKillResult{}, nil }
