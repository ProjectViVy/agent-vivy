package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/runtime"
)

func TestAppUsesInjectedLoggerAtComposition(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	var previousOutput, ownedOutput bytes.Buffer
	previous := slog.New(slog.NewTextHandler(&previousOutput, nil))
	oldDefault := slog.Default()
	slog.SetDefault(previous)
	t.Cleanup(func() { slog.SetDefault(oldDefault) })
	owned := slog.New(slog.NewTextHandler(&ownedOutput, nil))

	cfg := newDeepSeekTestConfig(t)
	if _, err := settings.Save(settings.Path(cfg.DataDirectory()), settings.Settings{Provider: "deepseek"}); err != nil {
		t.Fatal(err)
	}
	a, err := New(context.Background(), cfg, WithLogger(owned), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	if !strings.Contains(ownedOutput.String(), "settings overlay applied") {
		t.Fatalf("composition log missing from injected logger: %q", ownedOutput.String())
	}
	if previousOutput.Len() != 0 {
		t.Fatalf("composition leaked into previous process logger: %q", previousOutput.String())
	}
	if slog.Default() != previous {
		t.Fatal("App composition replaced the process default logger")
	}
}
