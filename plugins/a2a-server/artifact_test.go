package a2aserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// TestA2AArtifactClientSmoke is the official-client half of the packed-
// artifact gate (A2A-06 plan Steps 3–4). It is env-gated: the sdk
// artifact driver starts a real packed vivy binary and exports
// VIVY_A2A_SMOKE_BASE (the channel listener origin) and
// VIVY_A2A_SMOKE_TOKEN (its bearer). Without them the test skips so
// ordinary plugin unit runs stay self-contained.
func TestA2AArtifactClientSmoke(t *testing.T) {
	base := strings.TrimRight(os.Getenv("VIVY_A2A_SMOKE_BASE"), "/")
	token := os.Getenv("VIVY_A2A_SMOKE_TOKEN")
	if base == "" || token == "" {
		t.Skip("VIVY_A2A_SMOKE_BASE/VIVY_A2A_SMOKE_TOKEN unset; run under the sdk artifact driver")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Card discovery is the unauthenticated front door.
	resp, err := http.Get(base + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatalf("card fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("card status=%d", resp.StatusCode)
	}
	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("card decode: %v", err)
	}
	var rpcURL string
	for _, iface := range card.SupportedInterfaces {
		if iface.ProtocolBinding == a2a.TransportProtocolJSONRPC {
			rpcURL = iface.URL
		}
	}
	if rpcURL == "" {
		t.Fatalf("card has no jsonrpc interface: %+v", card.SupportedInterfaces)
	}

	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: &bearerTransport{token: token, next: http.DefaultTransport},
	}
	client, err := a2aclient.NewFromCard(ctx, &a2a.AgentCard{
		Name:         "artifact-smoke",
		Capabilities: a2a.AgentCapabilities{Streaming: true},
		SupportedInterfaces: []*a2a.AgentInterface{{
			URL:             rpcURL,
			ProtocolBinding: a2a.TransportProtocolJSONRPC,
			ProtocolVersion: a2a.Version,
		}},
	}, a2aclient.WithJSONRPCTransport(httpClient))
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	t.Run("bearer required on rpc", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL,
			strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ListTasks","params":{}}`))
		req.Header.Set("Content-Type", "application/json")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated rpc status=%d", r.StatusCode)
		}
	})

	var taskID a2a.TaskID
	t.Run("tool-bearing task reaches terminal committed state", func(t *testing.T) {
		msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("use the governed tool then answer with the artifact marker"))
		msg.ID = "artifact-smoke-1"
		var sawCompleted bool
		for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: msg}) {
			if err != nil {
				t.Fatalf("stream event error: %v", err)
			}
			var st a2a.TaskState
			var id a2a.TaskID
			switch e := ev.(type) {
			case *a2a.Task:
				st, id = e.Status.State, e.ID
			case *a2a.TaskStatusUpdateEvent:
				st, id = e.Status.State, e.TaskID
			default:
				continue
			}
			if taskID == "" {
				taskID = id
			}
			if st == a2a.TaskStateCompleted {
				sawCompleted = true
			}
		}
		if taskID == "" {
			t.Fatal("no task events")
		}
		if !sawCompleted {
			t.Fatalf("stream ended before TASK_STATE_COMPLETED (task %s)", taskID)
		}
		hl := 50
		got, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: taskID, HistoryLength: &hl})
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if got.Status.State != a2a.TaskStateCompleted {
			t.Fatalf("final state=%s", got.Status.State)
		}
		var found bool
		for _, m := range got.History {
			for _, p := range m.Parts {
				if strings.Contains(p.Text(), "artifact answer") {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("committed output missing artifact marker: %+v", got.History)
		}
	})

	t.Run("message id retry dedupes to one admission", func(t *testing.T) {
		// A message-id retry must replay the identical payload; a
		// conflicting body is correctly refused upstream.
		msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("use the governed tool then answer with the artifact marker"))
		msg.ID = "artifact-smoke-1"
		res, err := client.SendMessage(ctx, &a2a.SendMessageRequest{Message: msg})
		if err != nil {
			t.Fatalf("retry send: %v", err)
		}
		task, ok := res.(*a2a.Task)
		if !ok || task.ID != taskID {
			t.Fatalf("retry did not replay the committed task: %+v", res)
		}
	})
}

// bearerTransport injects the channel envelope's bearer credential on
// every transport request.
type bearerTransport struct {
	token string
	next  http.RoundTripper
}

func (b *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}
