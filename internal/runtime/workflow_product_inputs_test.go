package runtime

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	inofy "github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

// inofyBoundInputsDefinition is the two-node fixture whose dependent node
// authors all three binding kinds on the canonical `node.inputs` field:
// a literal, the run's root input packet, and an upstream node result.
const inofyBoundInputsDefinition = `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"PG upstream","tool_names":["echo_info"]}},
	{"id":"join","kind":"call","type":"vivy.child-task@1","config":{"task":"combine halves","tool_names":["echo_info"]},"inputs":{
		"literal_text":{"literal":"PG literal"},
		"root_text":{"source":"input","pointer":"/topic"},
		"upstream":{"source":"a","pointer":"/result"}
	}}
],"edges":[{"from":"a","to":"join"}],"exits":["join"],"outputs":{"answer":{"source":"join","pointer":"/result"}}}}`

// childRequestText returns the governed child task text the executor submitted
// for one child Run (the event that proves what the model actually received).
func childRequestText(t *testing.T, backend *sqlite.Backend, runID domain.RunID) string {
	t.Helper()
	task := ""
	for _, event := range replayAll(t, backend, runID) {
		if event.Type != domain.EventChildRequested {
			continue
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("child request payload: %v", err)
		}
		task = payload.Text
	}
	return task
}

// TestWorkflowProductInputBindingRoundTrip proves the canonical authoring shape
// survives draft → publish → admitted run: the engine resolves literal, root
// and upstream bindings from `node.inputs`, the dependent child receives every
// resolved value as labelled untrusted data, and a later draft edit cannot
// move the published revision.
func TestWorkflowProductInputBindingRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-inputs")
	parentRunID := prepareProductSession(t, svc, backend, author)

	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-inputs", definitions.ETagAbsent, inofyProductArtifact(inofyBoundInputsDefinition))
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	rev, err := svc.INOFYPublishDraft(ctx, author, "wf-inputs", draft.ETag)
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}

	published, err := svc.INOFYGetRevision(ctx, author, "wf-inputs", rev.Revision)
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	var join *inofy.Node
	for i := range published.Artifact.Definition.Graph.Nodes {
		if published.Artifact.Definition.Graph.Nodes[i].ID == "join" {
			join = &published.Artifact.Definition.Graph.Nodes[i]
		}
	}
	if join == nil {
		t.Fatal("published artifact lost the join node")
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(join.Config, &config); err != nil {
		t.Fatalf("join config: %v", err)
	}
	if _, exists := config["inputs"]; exists {
		t.Fatalf("published config duplicated the binding home: %s", config["inputs"])
	}
	if len(config) != 2 {
		t.Fatalf("published config kept unexpected fields: %v", config)
	}
	if len(join.Inputs) != 3 {
		t.Fatalf("published inputs = %+v", join.Inputs)
	}
	if root := join.Inputs["root_text"]; root.Source != "input" || root.Pointer != "/topic" || !root.HasPointer {
		t.Fatalf("root binding = %+v", root)
	}
	if upstream := join.Inputs["upstream"]; upstream.Source != "a" || upstream.Pointer != "/result" {
		t.Fatalf("upstream binding = %+v", upstream)
	}
	if literal := join.Inputs["literal_text"]; string(literal.Literal) != `"PG literal"` {
		t.Fatalf("literal binding = %+v", literal)
	}

	started, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-op-inputs", WorkflowID: "wf-inputs",
		Revision: rev.Revision, Input: json.RawMessage(`{"topic":"PG root"}`),
	})
	if err != nil || !started.Created {
		t.Fatalf("start: %v %+v", err, started)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)

	view, err := svc.INOFYGetRun(ctx, author, started.Run.ID)
	if err != nil || view.Details.EngineStatus != "succeeded" {
		t.Fatalf("run view: %v %+v", err, view)
	}

	children, err := backend.ListChildRuns(ctx, started.Run.ID)
	if err != nil || len(children) != 2 {
		t.Fatalf("children = %+v err=%v", children, err)
	}
	joinTask := ""
	for _, child := range children {
		if text := childRequestText(t, backend, child.ID); strings.Contains(text, "PG literal") {
			joinTask = text
		}
	}
	if joinTask == "" {
		t.Fatalf("no child received the resolved bindings: %+v", children)
	}
	for _, want := range []string{"PG literal", "PG root", "PG upstream", "untrusted"} {
		if !strings.Contains(joinTask, want) {
			t.Fatalf("dependent child task missing %q: %q", want, joinTask)
		}
	}
	output, err := svc.INOFYNodeOutput(ctx, author, started.Run.ID, "join")
	if err != nil || !strings.Contains(string(output), "combine halves") {
		t.Fatalf("join node output = %q err=%v", output, err)
	}

	// A later draft edit is a new draft artifact; revision 1 stays fixed.
	loadedDraft, err := svc.INOFYLoadDraft(ctx, author, "wf-inputs")
	if err != nil {
		t.Fatalf("load draft: %v", err)
	}
	edited := strings.Replace(inofyBoundInputsDefinition, "PG literal", "PG literal v2", 1)
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-inputs", loadedDraft.ETag, inofyProductArtifact(edited)); err != nil {
		t.Fatalf("edit draft: %v", err)
	}
	again, err := svc.INOFYGetRevision(ctx, author, "wf-inputs", rev.Revision)
	if err != nil {
		t.Fatalf("re-read revision: %v", err)
	}
	if again.DefinitionDigest != published.DefinitionDigest || !reflect.DeepEqual(again.Artifact, published.Artifact) {
		t.Fatal("a draft edit moved the published revision")
	}
}
