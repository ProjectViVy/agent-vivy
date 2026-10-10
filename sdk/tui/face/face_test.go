package face

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	faceport "agent-vivy/sdk/port/face"
	"agent-vivy/sdk/tui/live"
	"agent-vivy/sdk/tui/surface"
	"agent-vivy/sdk/tui/view"
)

type testEnv struct{ called bool }

func (*testEnv) ModuleID() string { return "vivy/tui" }

func (e *testEnv) Call(context.Context, string, any) (json.RawMessage, error) {
	e.called = true
	return json.RawMessage(`{}`), nil
}

func (*testEnv) OnEvent(func(string, json.RawMessage)) {}

func TestKind(t *testing.T) {
	if got := New(faceport.Options{}).Kind(); got != Kind {
		t.Fatalf("kind = %q", got)
	}
}

func TestRejectsNonTerminalBeforeInitialize(t *testing.T) {
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	env := &testEnv{}
	_, err = New(faceport.Options{Out: out, Err: io.Discard}).Run(context.Background(), env)
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("expected terminal error, got %v", err)
	}
	if env.called {
		t.Fatal("face initialized before validating its terminal")
	}
}

type localeEnv struct {
	calls  []string
	locale string
}

func (*localeEnv) ModuleID() string { return "vivy/tui" }

func (e *localeEnv) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	e.calls = append(e.calls, method)
	if method == "settings/get" {
		return json.Marshal(map[string]any{"locale": e.locale, "generation_locale": "en", "workspace_locale": e.locale, "locale_read_only": false})
	}
	return json.RawMessage(`{"capabilities":[]}`), nil
}

func (*localeEnv) OnEvent(func(string, json.RawMessage)) {}

func TestFaceSettingsLocaleReachesView(t *testing.T) {
	for _, locale := range []string{"en", "zh"} {
		t.Run(locale, func(t *testing.T) {
			env := &localeEnv{locale: locale}
			rendered := false
			f := &terminalFace{opts: faceport.Options{Out: io.Discard, Err: io.Discard, DebugToolOutput: true}}
			f.runView = func(driver surface.Driver, _ io.Writer, options ...view.Options) error {
				rendered = true
				if string(driver.(*live.Live).Locale()) != locale || len(options) != 1 || string(options[0].Locale) != locale || !options[0].DebugToolOutput {
					t.Fatalf("launch options=%+v", options)
				}
				model := view.New(driver, options...)
				updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
				want := "Ask something"
				if locale == "zh" {
					want = "问点什么"
				}
				if !strings.Contains(updated.View(), want) {
					t.Fatalf("localized composer missing: %s", updated.View())
				}
				return nil
			}
			result, err := f.Run(context.Background(), env)
			if err != nil || result.Status != "completed" || !rendered || strings.Join(env.calls, ",") != "initialize,settings/get" {
				t.Fatalf("result=%+v err=%v calls=%v rendered=%t", result, err, env.calls, rendered)
			}
		})
	}
}

type rpcEnv struct {
	turns map[string]map[string]any
}

func (*rpcEnv) ModuleID() string { return "vivy/tui" }

func (e *rpcEnv) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	if method == "initialize" {
		return json.RawMessage(`{"capabilities":[]}`), nil
	}
	if strings.HasPrefix(method, "turn/") {
		raw, _ := json.Marshal(params)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		e.turns[method] = decoded
		return json.RawMessage(`{"queued":true}`), nil
	}
	return json.RawMessage(`{}`), nil
}

func (*rpcEnv) OnEvent(func(string, json.RawMessage)) {}

// A queued turn degrades to a fresh run on an idle session; the kernel
// fallback runs RunWithOptions with the params' face. The code face must
// keep its "code" attribution — an empty face becomes a web run, which a
// persona-gated composition rejects.
func TestRPCModeQueuedTurnsKeepCodeFace(t *testing.T) {
	in := strings.NewReader(
		`{"id":"s1","type":"steer","message":"x"}` + "\n" +
			`{"id":"f1","type":"follow_up","message":"y"}` + "\n",
	)
	var out bytes.Buffer
	env := &rpcEnv{turns: map[string]map[string]any{}}
	f := &terminalFace{opts: faceport.Options{In: in, Out: &out, Err: io.Discard, SessionID: "sess-1"}}
	result, err := f.runRPCMode(context.Background(), env)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%+v err=%v out=%s", result, err, out.String())
	}
	for _, method := range []string{"turn/steer", "turn/follow_up"} {
		params, ok := env.turns[method]
		if !ok {
			t.Fatalf("%s was never issued", method)
		}
		if params["face"] != "code" {
			t.Fatalf("%s face = %v, want code", method, params["face"])
		}
		if params["session_id"] != "sess-1" {
			t.Fatalf("%s session_id = %v, want sess-1", method, params["session_id"])
		}
	}
}

func TestRPCQueuedTurnCarriesFullSubmission(t *testing.T) {
	in := strings.NewReader(`{"id":"q","type":"follow_up","message":"look","thinking":"high","mode":"plan","attachment_paths":["a.png"],"context_paths":["a.txt"]}` + "\n")
	var out bytes.Buffer
	env := &rpcEnv{turns: map[string]map[string]any{}}
	f := &terminalFace{opts: faceport.Options{In: in, Out: &out, Err: io.Discard, SessionID: "sess-1"}}
	if _, err := f.runRPCMode(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	params := env.turns["turn/follow_up"]
	if params["thinking"] != "high" || params["mode"] != "plan" || params["attachment_paths"] == nil || params["context_paths"] == nil {
		t.Fatalf("lost queued options: %#v", params)
	}
}
