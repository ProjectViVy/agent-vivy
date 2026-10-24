package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/storage/migrations"
)

func TestIncompatibleStoreHintNamesTheDataDirectory(t *testing.T) {
	err := fmt.Errorf("storage module: open storage: %w", migrations.ErrUnsupportedSourceFormat)
	hint, ok := incompatibleStoreHint(err, "/home/u/.vivy")
	if !ok {
		t.Fatal("want hint for unsupported source format")
	}
	if !strings.Contains(hint, "/home/u/.vivy") {
		t.Fatalf("hint %q does not name the data directory", hint)
	}
	if !strings.Contains(hint, "VIVY_USER_HOME") {
		t.Fatalf("hint %q does not name the override", hint)
	}
}

func TestIncompatibleStoreHintCoversUpgradeRequired(t *testing.T) {
	err := fmt.Errorf("storage module: open storage: %w", migrations.ErrUpgradeRequired)
	if _, ok := incompatibleStoreHint(err, "dir"); !ok {
		t.Fatal("want hint for upgrade required")
	}
}

func TestIncompatibleStoreHintStaysQuietForOtherFailures(t *testing.T) {
	if hint, ok := incompatibleStoreHint(errors.New("storage module: occupy shared workspace: busy"), "dir"); ok || hint != "" {
		t.Fatalf("unexpected hint %q for an unrelated failure", hint)
	}
}
