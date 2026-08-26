package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

func TestApplySettingsOverlayExecuteTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := config.Default()
	if base.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("config default ceiling = %d, want 30", base.Runtime.ExecuteMaxTimeoutSeconds)
	}
	// Point the data root at a scratch dir so the test never reads or writes
	// a real agent-home document.
	base.Storage.SQLite.Path = filepath.Join(t.TempDir(), "vivy.db")

	// No document: the config value stands.
	got := applySettingsOverlay(context.Background(), logger, base)
	if got.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("empty overlay changed ceiling to %d", got.Runtime.ExecuteMaxTimeoutSeconds)
	}

	// A persisted override wins.
	if _, err := settings.Save(settings.Path(base.DataDirectory()), settings.Settings{ExecuteMaxTimeoutSeconds: 300}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, base)
	if got.Runtime.ExecuteMaxTimeoutSeconds != 300 {
		t.Fatalf("overlay ceiling = %d, want 300", got.Runtime.ExecuteMaxTimeoutSeconds)
	}
}
