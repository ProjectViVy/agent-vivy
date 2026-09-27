package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

const (
	d15CrashWorkerEnv  = "VIVY_D15_CRASH_WORKER"
	d15CrashDBEnv      = "VIVY_D15_CRASH_DB"
	d15CrashRunEnv     = "VIVY_D15_CRASH_RUN"
	d15CrashSessionEnv = "VIVY_D15_CRASH_SESSION"
	d15CrashOpEnv      = "VIVY_D15_CRASH_OPERATION"
	d15CrashRequestEnv = "VIVY_D15_CRASH_REQUEST"
	d15CrashEffectEnv  = "VIVY_D15_CRASH_EFFECT"
	d15CrashReadyEnv   = "VIVY_D15_CRASH_READY"
	d15CrashStageEnv   = "VIVY_D15_CRASH_STAGE"
)

const (
	d15CrashBeforeClaim                = "before-claim"
	d15CrashClaimBeforeInvoke          = "claim-before-invoke"
	d15CrashEffectBeforeCompletion     = "effect-before-completion"
	d15CrashCompletionBeforeCheckpoint = "completion-before-checkpoint"
)

func TestToolOperationCrashMatrix(t *testing.T) {
	cases := []struct {
		name               string
		stage              string
		stateAfterCrash    domain.ToolOperationState
		effectsAfterCrash  int
		wantRecoveryOutput string
		wantUnknown        bool
	}{
		{name: "admitted before claim", stage: d15CrashBeforeClaim, stateAfterCrash: domain.ToolOperationAdmitted, effectsAfterCrash: 0, wantRecoveryOutput: "external-effect-result"},
		{name: "claimed before invoke", stage: d15CrashClaimBeforeInvoke, stateAfterCrash: domain.ToolOperationClaimed, effectsAfterCrash: 0, wantUnknown: true},
		{name: "effect before completion", stage: d15CrashEffectBeforeCompletion, stateAfterCrash: domain.ToolOperationClaimed, effectsAfterCrash: 1, wantUnknown: true},
		{name: "completion before graph checkpoint", stage: d15CrashCompletionBeforeCheckpoint, stateAfterCrash: domain.ToolOperationCompleted, effectsAfterCrash: 1, wantRecoveryOutput: "external-effect-result"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newD15CrashFixture(t)
			if err := fixture.backend.Close(); err != nil {
				t.Fatalf("close setup store before crash worker: %v", err)
			}
			readyPath := filepath.Join(filepath.Dir(fixture.dbPath), "worker-ready")
			cmd := exec.Command(os.Args[0], "-test.run=^TestToolOperationCrashWorker$")
			cmd.Env = append(os.Environ(),
				d15CrashWorkerEnv+"=1",
				d15CrashDBEnv+"="+fixture.dbPath,
				d15CrashRunEnv+"="+string(fixture.runID),
				d15CrashSessionEnv+"="+string(fixture.sessionID),
				d15CrashOpEnv+"="+fixture.operationID,
				d15CrashRequestEnv+"="+string(fixture.request),
				d15CrashEffectEnv+"="+fixture.effectPath,
				d15CrashReadyEnv+"="+readyPath,
				d15CrashStageEnv+"="+tc.stage,
			)
			var output strings.Builder
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatalf("start crash worker: %v", err)
			}
			wait := make(chan error, 1)
			go func() { wait <- cmd.Wait() }()
			ready, timedOut := waitForCrashReady(t, readyPath)
			select {
			case <-wait:
				t.Fatalf("crash worker exited before its kill boundary: %s", output.String())
			case <-ready:
			case <-timedOut:
				_ = cmd.Process.Kill()
				<-wait
				t.Fatal("crash worker did not reach its injected boundary within 10 seconds")
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatalf("kill crash worker: %v", err)
			}
			if err := <-wait; err == nil {
				t.Fatal("crash worker exited successfully; test did not terminate the process")
			}

			service, backend := openD15CrashService(t, fixture.dbPath)
			defer backend.Close()
			operation, err := backend.GetToolOperation(context.Background(), fixture.runID, fixture.operationID)
			if err != nil || operation.State != tc.stateAfterCrash {
				t.Fatalf("operation after process crash = %+v/%v, want state %s", operation, err, tc.stateAfterCrash)
			}
			effectsAfterCrash := countD15CrashEffects(t, fixture.effectPath)
			if effectsAfterCrash != tc.effectsAfterCrash {
				t.Fatalf("external effects after crash = %d, want %d", effectsAfterCrash, tc.effectsAfterCrash)
			}
			ctx, err := d15CrashRunContext(context.Background(), service, backend, fixture.runID, fixture.sessionID)
			if err != nil {
				t.Fatalf("restore Service context: %v", err)
			}
			result, runErr := invokeD15CrashWorkflow(ctx, service, fixture.runID, fixture.sessionID,
				fixture.operationID, fixture.request, fixture.effectPath, "", "")
			if tc.wantUnknown {
				if !errors.Is(runErr, ErrToolOperationUnknown) {
					t.Fatalf("recovery result = %q/%v, want visibly blocked unknown operation", result, runErr)
				}
			} else if runErr != nil || result != tc.wantRecoveryOutput {
				t.Fatalf("recovery result = %q/%v, want %q", result, runErr, tc.wantRecoveryOutput)
			}
			wantEffects := tc.effectsAfterCrash
			if !tc.wantUnknown && tc.effectsAfterCrash == 0 {
				wantEffects = 1
			}
			if got := countD15CrashEffects(t, fixture.effectPath); got != wantEffects {
				t.Fatalf("external effects after recovery = %d, want %d", got, wantEffects)
			}
			events := replayEvents(t, backend, fixture.runID)
			var operationSequences []domain.EventSeq
			for _, event := range events {
				if event.Type == domain.EventToolOperation {
					operationSequences = append(operationSequences, event.Seq)
				}
			}
			t.Logf("run=%s operation=%s checkpoint=%s operation_event_sequences=%v effects=%d recovery=%q/%v",
				fixture.runID, fixture.operationID, checkpointIDFor(fixture.runID), operationSequences, wantEffects, result, runErr)
		})
	}
}

func TestToolOperationCrashWorker(t *testing.T) {
	if os.Getenv(d15CrashWorkerEnv) != "1" {
		return
	}
	stage := os.Getenv(d15CrashStageEnv)
	readyPath := os.Getenv(d15CrashReadyEnv)
	if stage == d15CrashBeforeClaim {
		signalAndBlockD15CrashWorker(t, readyPath)
	}
	dbPath := os.Getenv(d15CrashDBEnv)
	service, backend := openD15CrashService(t, dbPath)
	defer backend.Close()
	if stage == d15CrashClaimBeforeInvoke {
		service.deps.ToolOperations = d15CrashPauseClaimStore{
			ToolOperationStore: backend, readyPath: readyPath,
		}
	}
	ctx, err := d15CrashRunContext(context.Background(), service, backend,
		domain.RunID(os.Getenv(d15CrashRunEnv)), domain.SessionID(os.Getenv(d15CrashSessionEnv)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = invokeD15CrashWorkflow(ctx, service, domain.RunID(os.Getenv(d15CrashRunEnv)), domain.SessionID(os.Getenv(d15CrashSessionEnv)),
		os.Getenv(d15CrashOpEnv), []byte(os.Getenv(d15CrashRequestEnv)), os.Getenv(d15CrashEffectEnv), stage, readyPath)
	if err != nil {
		t.Fatalf("crash worker workflow: %v", err)
	}
	t.Fatalf("crash worker unexpectedly completed before injected termination at %q", stage)
}

type d15CrashFixture struct {
	dbPath      string
	effectPath  string
	sessionID   domain.SessionID
	runID       domain.RunID
	operationID string
	request     []byte
	backend     *sqlite.Backend
}

func newD15CrashFixture(t *testing.T) d15CrashFixture {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "d15-crash.db")
	service, backend := openD15CrashService(t, dbPath)
	sessionID := domain.SessionID("session-d15-crash")
	runID := domain.RunID("run-d15-crash")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "D15 crash", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create crash-test session: %v", err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: runID, GenerationID: service.deps.GenerationID,
		Capture: maskcontract.Capture{Selection: maskcontract.Selection{SessionID: sessionID}},
	})
	if err != nil {
		t.Fatalf("build crash-test prompt: %v", err)
	}
	mapper := newEventMapper(runID, 64<<10)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: "test", Model: "test-model", Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(domain.PolicyProfileDefault), PolicyHash: "d15-crash-policy",
		PromptSchema: prompt.SchemaVersion, PromptDigest: prompt.PayloadSHA256,
	})
	_, err = backend.CommitRunAdmission(ctx, storage.RunAdmission{
		Message: domain.Message{ID: newMessageID(), SessionID: sessionID, RunID: runID, Role: domain.RoleUser,
			CreatedAt: time.Now().UnixMilli(), Content: "D15 crash recovery", Source: "workflow"},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli()},
		Started: started, Prompt: &prompt,
	})
	if err != nil {
		t.Fatalf("admit crash-test Run: %v", err)
	}
	request := []byte(`{"effect":"write once"}`)
	operationID := "d15-crash-effect-1"
	ctx = withRunPrompt(ctx, prompt)
	coordinator := service.newToolOperationCoordinator(runID, sessionID)
	if _, err := coordinator.Admit(ctx, operationID, "crash.effect", request, request, request); err != nil {
		t.Fatalf("admit crash-test operation: %v", err)
	}
	return d15CrashFixture{
		dbPath: dbPath, effectPath: filepath.Join(filepath.Dir(dbPath), "external-effects.log"),
		sessionID: sessionID, runID: runID, operationID: operationID, request: request, backend: backend,
	}
}

func openD15CrashService(t *testing.T, dbPath string) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open D15 crash-test store: %v", err)
	}
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		_ = backend.Close()
		t.Fatalf("resolve crash-test tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		_ = backend.Close()
		t.Fatalf("create crash-test checkpoint store: %v", err)
	}
	engine, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		_ = backend.Close()
		t.Fatalf("create crash-test Engine: %v", err)
	}
	service := NewService(engine, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		Admission: backend, GenerationID: "d15-crash-generation", ToolOperations: backend,
	})
	return service, backend
}

func d15CrashRunContext(ctx context.Context, service *Service, backend *sqlite.Backend, runID domain.RunID, sessionID domain.SessionID) (context.Context, error) {
	prompt, err := backend.LoadRunPrompt(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load crash-test prompt: %w", err)
	}
	ctx = withRunPrompt(ctx, prompt)
	ctx = withRunID(withSessionID(ctx, sessionID), runID)
	return withToolOperationCoordinator(ctx, service.newToolOperationCoordinator(runID, sessionID)), nil
}

func invokeD15CrashWorkflow(ctx context.Context, service *Service, runID domain.RunID, sessionID domain.SessionID,
	operationID string, request []byte, effectPath, pauseStage, readyPath string) (string, error) {
	workflow := compose.NewWorkflow[string, string]()
	workflow.AddLambdaNode("effect", compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		coordinator, ok := toolOperationCoordinatorFromContext(ctx).(serviceToolOperationCoordinator)
		if !ok || coordinator.service != service {
			return "", ErrToolOperationUnavailable
		}
		operation, found, err := coordinator.Lookup(ctx, operationID, "crash.effect", request)
		if err != nil {
			return "", err
		}
		if !found {
			operation, err = coordinator.Admit(ctx, operationID, "crash.effect", request, request, request)
			if err != nil {
				return "", err
			}
		}
		result, err := coordinator.Execute(ctx, operation, func(context.Context) (string, error) {
			if err := appendD15CrashEffect(effectPath); err != nil {
				return "", err
			}
			if pauseStage == d15CrashEffectBeforeCompletion {
				signalAndBlockD15CrashWorker(nil, readyPath)
			}
			return "external-effect-result", nil
		})
		if err != nil {
			return "", err
		}
		if pauseStage == d15CrashCompletionBeforeCheckpoint {
			signalAndBlockD15CrashWorker(nil, readyPath)
		}
		return result, nil
	})).AddInput(compose.START)
	workflow.End().AddInput("effect")
	runnable, err := workflow.Compile(ctx,
		compose.WithGraphName("vivy-d15-crash-workflow"),
		compose.WithCheckPointStore(NewEinoCheckpointAdapter(service.engine.cfg.Checkpoints)),
	)
	if err != nil {
		return "", fmt.Errorf("compile D15 crash workflow: %w", err)
	}
	return runnable.Invoke(ctx, "external effect", compose.WithCheckPointID(checkpointIDFor(runID)))
}

type d15CrashPauseClaimStore struct {
	storage.ToolOperationStore
	readyPath string
}

func (store d15CrashPauseClaimStore) ClaimToolOperation(ctx context.Context, runID domain.RunID, operationID, owner string) (domain.ToolOperation, bool, domain.RunEvent, error) {
	operation, acquired, event, err := store.ToolOperationStore.ClaimToolOperation(ctx, runID, operationID, owner)
	if err == nil && acquired {
		signalAndBlockD15CrashWorker(nil, store.readyPath)
	}
	return operation, acquired, event, err
}

func signalAndBlockD15CrashWorker(t *testing.T, readyPath string) {
	if t != nil {
		t.Helper()
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		if t != nil {
			t.Fatalf("signal crash worker boundary: %v", err)
		}
		panic(err)
	}
	select {}
}

func waitForCrashReady(t *testing.T, readyPath string) (<-chan struct{}, <-chan struct{}) {
	t.Helper()
	ready := make(chan struct{})
	timedOut := make(chan struct{})
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(readyPath); err == nil {
				close(ready)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		close(timedOut)
	}()
	return ready, timedOut
}

func appendD15CrashEffect(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, "effect")
	return err
}

func countD15CrashEffects(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read external effect fixture: %v", err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			count++
		}
	}
	return count
}
