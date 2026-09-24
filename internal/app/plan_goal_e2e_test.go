package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/migrations"
	"agent-vivy/internal/tools"

	_ "modernc.org/sqlite"
)

// A dropped model-to-tool Plan submission must make the pending review
// invisible here, even though the RPC and storage remain healthy.
func TestPlanGoalIntegratedReviewAndTwoRounds(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "plan-goal-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	dir := t.TempDir()
	cachePath := os.Getenv("GOCACHE")
	if cachePath == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			t.Fatalf("resolve Go build cache: %v", err)
		}
		cachePath = filepath.Join(userCache, "go-build")
	}
	model := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.WriteFileName, args: `{"path":"PLAN.md","content":"1. Write the change.\n2. Verify it.\n"}`},
		{tool: tools.SubmitPlanName, args: `{"markdown":"1. Write the change.\n2. Verify it."}`},
		{text: "The plan was approved."},
		{tool: tools.WriteFileName, args: `{"path":"calc.go","content":"package calc\n\nfunc Answer() int { return 42 }\n"}`},
		{text: "The code is changed; validate it next."},
		{tool: tools.ExecuteName, args: fmt.Sprintf(`{"command":"go","args":["test","./..."],"timeout_ms":60000,"env":{"GOCACHE":%q}}`, cachePath)},
		{tool: tools.ReportGoalName, args: `{"goal_id":"goal-e2e","revision":1,"status":"completed","reason":"go test passed"}`},
		{text: "Goal complete."},
	})
	t.Setenv("VIVY_API_BASE", model.URL)
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"go.mod":       "module example.com/calc\n\ngo 1.23\n",
		"calc.go":      "package calc\n\nfunc Answer() int { return 0 }\n",
		"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) { if Answer() != 42 { t.Fatal(\"wrong answer\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", filepath.Join(goruntime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfg := config.Config{
		Server:    config.Server{Addr: "127.0.0.1:0"},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(dir, "vivy.db")}},
		Providers: config.Providers{Active: "deepseek"},
		Runtime:   config.Runtime{StreamBuffer: 16, MaxEventPayloadBytes: 64 << 10, WorkspaceRoot: workspace, ExecuteAllowedCommands: []string{"go"}},
		Tools:     config.Tools{Enabled: []string{tools.WriteFileName, tools.SubmitPlanName, tools.ExecuteName, tools.ReportGoalName}, Approval: config.Approval{Expiration: time.Minute}},
	}
	cfg.Runtime.Sandbox.Approval.AutoApproveTools = []string{tools.WriteFileName, tools.SubmitPlanName, tools.ExecuteName, tools.ReportGoalName}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial real control plane: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "plan-goal integration", "workspace_path": workspace})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create = %v", session)
	}
	callControl(t, client, "session/set_permission", map[string]any{"session_id": sessionID, "preset": "trusted"})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-plan", "expected_version": 0})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Prepare the design for review."})
	runID, _ := turn["run_id"].(string)
	if runID == "" {
		t.Fatalf("turn/start = %v", turn)
	}
	var work map[string]any
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		work = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		plan, _ := work["plan"].(map[string]any)
		if plan["review_status"] == "pending" && work["version"].(float64) >= 3 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if work["plan"].(map[string]any)["review_status"] != "pending" || work["version"].(float64) < 3 {
		t.Fatalf("no pending review: work=%v run=%v events=%v", work, callControl(t, client, "run/get", map[string]any{"run_id": runID}), runEvents(t, client, runID))
	}
	plan := work["plan"].(map[string]any)
	if plan["markdown"] != "1. Write the change.\n2. Verify it." || plan["origin_run_id"] != runID {
		t.Fatalf("durable review projection = %v, want exact submitted plan and originating run", plan)
	}
	if raw, err := os.ReadFile(filepath.Join(workspace, "PLAN.md")); err != nil || string(raw) != "1. Write the change.\n2. Verify it.\n" {
		t.Fatalf("Plan did not save design under writable policy: %q / %v", raw, err)
	}
	callControl(t, client, "plan/decide", map[string]any{
		"session_id": sessionID, "request_id": "approve-goal", "expected_version": work["version"],
		"submission_id": plan["submission_id"], "action": "start_goal", "goal_id": "goal-e2e",
		"objective": "Change Answer to 42 and validate with go test.", "max_rounds": 2,
	})
	completed := false
	deadline = time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		work = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		goal, _ := work["goal"].(map[string]any)
		if goal["phase"] == "completed" {
			completed = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !completed {
		runs, _ := a.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
		var lastEvents []map[string]any
		if len(runs) > 0 {
			lastEvents = runEvents(t, client, string(runs[len(runs)-1].ID))
		}
		t.Fatalf("Goal did not complete: work=%v runs=%v last_run_events=%v", work, runs, lastEvents)
	}
	goal := work["goal"].(map[string]any)
	if goal["rounds_started"] != float64(2) || goal["evidence_run_id"] == "" {
		t.Fatalf("Goal result = %v, want two admitted rounds and a real evidence run", goal)
	}
	if raw, err := os.ReadFile(filepath.Join(workspace, "calc.go")); err != nil || string(raw) != "package calc\n\nfunc Answer() int { return 42 }\n" {
		t.Fatalf("first Goal round did not change code: %q / %v", raw, err)
	}
	evidenceRunID := goal["evidence_run_id"].(string)
	events := runEvents(t, client, evidenceRunID)
	var tested bool
	for _, event := range events {
		if event["type"] == "tool.finished" {
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.ExecuteName && payload["error"] == nil {
				tested = true
			}
		}
	}
	if !tested {
		t.Fatalf("completed Goal evidence run %s has no successful execute event: %v", evidenceRunID, events)
	}
	waitFor(t, 5*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": evidenceRunID})
		return run["status"] == "completed"
	})
	before, err := a.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(before) != 3 {
		t.Fatalf("pre-restart runs = %v / %v, want Plan plus two ordinary Goal runs", before, err)
	}
	_ = client.Close()
	if err := a.Close(); err != nil {
		t.Fatalf("close app at restart boundary: %v", err)
	}
	restarted, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("restart real app: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	replay, err := restarted.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial restarted control plane: %v", err)
	}
	t.Cleanup(func() { _ = replay.Close() })
	replayed := callControl(t, replay, "session/work/get", map[string]any{"session_id": sessionID})
	replayedGoal, _ := replayed["goal"].(map[string]any)
	if replayed["activation"] != "disarmed" || replayedGoal["phase"] != "completed" || replayedGoal["evidence_run_id"] != evidenceRunID || replayedGoal["rounds_started"] != float64(2) {
		t.Fatalf("restart replay changed completed/disarmed Goal: %v", replayed)
	}
	after, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(after) != 3 {
		t.Fatalf("restart admitted extra run: %v / %v", after, err)
	}
}

// A committed review and its checkpoint survive process loss. Closing App
// gracefully cancels suspended runs, so closing only the database here models
// the crash boundary rather than an orderly application shutdown.
func TestPlanGoalIntegratedPendingReviewRecovery(t *testing.T) {
	planGoalTestProvider(t, newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.SubmitPlanName, args: `{"markdown":"Review this durable plan."}`},
		{text: "Review accepted."},
	}).URL)
	cfg, workspace := planGoalTestConfig(t, []string{tools.SubmitPlanName})
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	session := callControl(t, client, "session/create", map[string]any{"title": "review recovery", "workspace_path": workspace})
	sessionID := session["id"].(string)
	callControl(t, client, "session/set_permission", map[string]any{"session_id": sessionID, "preset": "trusted"})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-review", "expected_version": 0})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Submit a plan."})
	runID := turn["run_id"].(string)
	var pending map[string]any
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pending = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		plan, _ := pending["plan"].(map[string]any)
		state, err := a.backend.(storage.WorkStore).ReadWork(context.Background(), domain.SessionID(sessionID))
		if plan["review_status"] == "pending" && err == nil && state.Plan.ResumeTarget != "" {
			ready = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		state, err := a.backend.(storage.WorkStore).ReadWork(context.Background(), domain.SessionID(sessionID))
		t.Fatalf("review did not reach suspend boundary: view=%v state=%+v err=%v run=%v events=%v", pending, state.Plan, err, callControl(t, client, "run/get", map[string]any{"run_id": runID}), runEvents(t, client, runID))
	}
	if pending["plan"].(map[string]any)["origin_run_id"] != runID {
		t.Fatalf("pending review lacks originating run: %v", pending)
	}
	durablePending, err := a.backend.(storage.WorkStore).ReadWork(context.Background(), domain.SessionID(sessionID))
	if err != nil || durablePending.Plan.OriginToolCallID == "" || durablePending.Plan.ResumeTarget == "" {
		t.Fatalf("pending review lacks resumable origin: %+v / %v", durablePending.Plan, err)
	}
	_ = client.Close()
	if err := a.backend.Close(); err != nil {
		t.Fatalf("simulate process loss: %v", err)
	}
	// App.Close releases process-owned resources; its cancellation cannot write
	// to the already-closed Journal, matching an ungraceful process exit.
	_ = a.Close()
	restarted := openPlanGoalTestApp(t, cfg)
	replay, err := restarted.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	work := callControl(t, replay, "session/work/get", map[string]any{"session_id": sessionID})
	plan := work["plan"].(map[string]any)
	if plan["review_status"] != "pending" || plan["origin_run_id"] != runID || plan["markdown"] != "Review this durable plan." {
		t.Fatalf("recovered review = %v", work)
	}
	run := callControl(t, replay, "run/get", map[string]any{"run_id": runID})
	if run["status"] != "active" {
		t.Fatalf("recovered Plan run = %v, work = %v, before = %+v", run, work, durablePending.Plan)
	}
	before, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(before) != 1 {
		t.Fatalf("recovered runs = %v / %v", before, err)
	}
	callControl(t, replay, "plan/decide", map[string]any{
		"session_id": sessionID, "request_id": "approve-recovered", "expected_version": work["version"],
		"submission_id": plan["submission_id"], "action": "execute_once",
	})
	waitFor(t, 5*time.Second, func() bool {
		return callControl(t, replay, "run/get", map[string]any{"run_id": runID})["status"] == "completed"
	})
	after, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(after) != 1 {
		t.Fatalf("review replay admitted duplicate run: %v / %v", after, err)
	}
}

func TestPlanGoalIntegratedRoundLimitBlocksDurably(t *testing.T) {
	planGoalTestProvider(t, newPlanGoalScriptServer(t, []planGoalReply{{text: "One round ended without report_goal."}}).URL)
	cfg, workspace := planGoalTestConfig(t, nil)
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session := callControl(t, client, "session/create", map[string]any{"title": "round limit", "workspace_path": workspace})
	sessionID := session["id"].(string)
	callControl(t, client, "goal/create", map[string]any{
		"session_id": sessionID, "request_id": "cap-one", "expected_version": 0,
		"goal_id": "goal-cap-one", "goal_revision": 1, "objective": "Finish without a report.", "max_rounds": 1,
	})
	var work map[string]any
	waitFor(t, 5*time.Second, func() bool {
		work = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		goal, _ := work["goal"].(map[string]any)
		return goal["phase"] == "blocked"
	})
	goal := work["goal"].(map[string]any)
	if goal["reason"] != "goal round limit reached" || goal["rounds_started"] != float64(1) || work["activation"] != "disarmed" {
		t.Fatalf("cap did not persist explicit block: %v", work)
	}
	runs, err := a.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunCompleted || goal["evidence_run_id"] != string(runs[0].ID) {
		t.Fatalf("capped runs = %v / %v, work=%v", runs, err, work)
	}
	assertWorkEventCount(t, a, sessionID, domain.WorkEventGoalRoundAdmitted, 1)
	assertWorkEventCount(t, a, sessionID, domain.WorkEventGoalBlocked, 1)
	_ = client.Close()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openPlanGoalTestApp(t, cfg)
	replayed, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(replayed) != 1 {
		t.Fatalf("restart exceeded cap: %v / %v", replayed, err)
	}
}

func TestPlanGoalIntegratedPauseInFlightAndReopen(t *testing.T) {
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	}))
	t.Cleanup(model.Close)
	planGoalTestProvider(t, model.URL)
	cfg, workspace := planGoalTestConfig(t, nil)
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	session := callControl(t, client, "session/create", map[string]any{"title": "inflight pause", "workspace_path": workspace})
	sessionID := session["id"].(string)
	callControl(t, client, "goal/create", map[string]any{
		"session_id": sessionID, "request_id": "start-inflight", "expected_version": 0,
		"goal_id": "goal-inflight", "goal_revision": 1, "objective": "Wait for cancellation.", "max_rounds": 2,
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal never reached model request")
	}
	work := callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
	if work["activation"] != "armed" {
		t.Fatalf("inflight Goal not armed: %v", work)
	}
	callControl(t, client, "goal/pause", map[string]any{
		"session_id": sessionID, "request_id": "pause-inflight", "expected_version": work["version"],
		"goal_id": "goal-inflight", "goal_revision": 1, "reason": "user paused during model request",
	})
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("pause did not cancel in-flight model request")
	}
	waitFor(t, 5*time.Second, func() bool {
		view := callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		goal, _ := view["goal"].(map[string]any)
		return goal["phase"] == "paused" && view["activation"] == "disarmed"
	})
	_ = client.Close()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openPlanGoalTestApp(t, cfg)
	replay, err := restarted.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	recovered := callControl(t, replay, "session/work/get", map[string]any{"session_id": sessionID})
	goal := recovered["goal"].(map[string]any)
	if goal["phase"] != "paused" || recovered["activation"] != "disarmed" || goal["rounds_started"] != float64(1) {
		t.Fatalf("inflight pause lost across restart: %v", recovered)
	}
	runs, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(runs) != 1 || !runs[0].Status.Terminal() {
		t.Fatalf("inflight Journal = %v / %v", runs, err)
	}
	assertWorkEventCount(t, restarted, sessionID, domain.WorkEventGoalRoundAdmitted, 1)
	assertWorkEventCount(t, restarted, sessionID, domain.WorkEventGoalPaused, 1)
}

func TestPlanGoalIntegratedOpensVersion23SQLite(t *testing.T) {
	planGoalTestProvider(t, newPlanGoalScriptServer(t, nil).URL)
	cfg, _ := planGoalTestConfig(t, nil)
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+cfg.Storage.SQLite.Path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range manifest.Migrations(migrations.SQLite)[:23] {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(migration.SQL); err != nil {
			_ = tx.Rollback()
			t.Fatalf("seed migration %d: %v", migration.Version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)`, migration.Version, migration.Name, migration.Checksum, int64(1)); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO sessions (id,title,created_at) VALUES ('legacy-app-session','preserved',11)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session := callControl(t, client, "session/get", map[string]any{"session_id": "legacy-app-session"})
	if session["session"].(map[string]any)["title"] != "preserved" {
		t.Fatalf("old session not preserved by app.New migration: %v", session)
	}
	work := callControl(t, client, "session/work/get", map[string]any{"session_id": "legacy-app-session"})
	if work["version"] != float64(0) {
		t.Fatalf("upgraded Work projection: %v", work)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	check, err := sql.Open("sqlite", "file:"+cfg.Storage.SQLite.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var count int
	if err := check.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != len(manifest.Migrations(migrations.SQLite)) {
		t.Fatalf("app.New migration count = %d / %v", count, err)
	}
}

func planGoalTestProvider(t *testing.T, url string) {
	t.Helper()
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "plan-goal-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", url)
}

func planGoalTestConfig(t *testing.T, enabled []string) (config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := newDeepSeekTestConfig(t)
	cfg.Storage.SQLite.Path = filepath.Join(dir, "vivy.db")
	cfg.Runtime.WorkspaceRoot = workspace
	cfg.Tools.Enabled = enabled
	cfg.Runtime.Sandbox.Approval.AutoApproveTools = enabled
	return cfg, workspace
}

func openPlanGoalTestApp(t *testing.T, cfg config.Config) *App {
	t.Helper()
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func assertWorkEventCount(t *testing.T, a *App, sessionID string, kind domain.WorkEventKind, want int) {
	t.Helper()
	workStore, ok := a.backend.(storage.WorkStore)
	if !ok {
		t.Fatal("app backend lacks durable WorkStore")
	}
	events, _, err := workStore.ReplayWork(context.Background(), domain.SessionID(sessionID), domain.WorkState{SessionID: domain.SessionID(sessionID)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	for _, event := range events {
		if event.Kind == kind {
			got++
		}
	}
	if got != want {
		t.Fatalf("persisted %s count = %d, want %d: %v", kind, got, want, events)
	}
}

// An auto-approval preference cannot lift the read-only sandbox prohibition.
func TestPlanGoalIntegratedReadOnlyDeniesPlanWrite(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "plan-goal-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	model := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.WriteFileName, args: `{"path":"blocked.txt","content":"must not exist"}`},
		{text: "The write was denied."},
	})
	t.Setenv("VIVY_API_BASE", model.URL)
	workspace := filepath.Join(t.TempDir(), "read-only")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := newDeepSeekTestConfig(t)
	cfg.Runtime.WorkspaceRoot = workspace
	cfg.Tools.Enabled = []string{tools.WriteFileName}
	cfg.Runtime.Sandbox.Approval.AutoApproveTools = []string{tools.WriteFileName}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose read-only app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "read-only Plan", "workspace_path": workspace})
	sessionID := session["id"].(string)
	callControl(t, client, "session/set_permission", map[string]any{"session_id": sessionID, "preset": "cautious"})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-read-only-plan", "expected_version": 0})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Try writing during Plan."})
	runID := turn["run_id"].(string)
	waitFor(t, 5*time.Second, func() bool {
		return callControl(t, client, "run/get", map[string]any{"run_id": runID})["status"] == "completed"
	})
	if _, err := os.Stat(filepath.Join(workspace, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("read-only Plan wrote blocked file: %v", err)
	}
	var denied, approval bool
	for _, event := range runEvents(t, client, runID) {
		switch event["type"] {
		case "policy.evaluated":
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.WriteFileName && payload["decision"] == "deny" {
				denied = true
			}
		case "tool.approval_required":
			approval = true
		}
	}
	if !denied || approval {
		t.Fatalf("read-only write governance: denied=%v approval=%v", denied, approval)
	}
}

type planGoalReply struct{ tool, args, text string }

func newPlanGoalScriptServer(t *testing.T, replies []planGoalReply) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	next := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		if next >= len(replies) {
			mu.Unlock()
			http.Error(w, "script exhausted", http.StatusInternalServerError)
			return
		}
		reply := replies[next]
		next++
		callIndex := next
		mu.Unlock()
		if !request.Stream {
			http.Error(w, "expected streamed model request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(delta any, finish any) {
			chunk := map[string]any{"id": "chatcmpl-plan-goal", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
			encoded, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
		}
		if reply.tool != "" {
			write(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("pg-call-%d", callIndex), "type": "function", "function": map[string]any{"name": reply.tool, "arguments": ""}}}}, nil)
			write(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": reply.args}}}}, nil)
			write(map[string]any{}, "tool_calls")
		} else {
			write(map[string]any{"role": "assistant", "content": reply.text}, nil)
			write(map[string]any{}, "stop")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)
	return server
}
