package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMemoryLoopEffectHandshakeWaitsForWindowsSharingLock(t *testing.T) {
	want := memoryLoopEffectReceiptHandshake{PID: 123, OperationID: "fixture-operation", TargetRef: "fixture-memory", Revision: 2, Status: "applied"}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = syscall.CloseHandle(handle) }) }
	t.Cleanup(release)
	// ERROR_SHARING_VIOLATION: prove the real fixture file is temporarily unreadable.
	if _, err := os.ReadFile(path); !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("expected sharing violation while locked, got %v", err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		release()
	}()
	if got := waitMemoryLoopEffectHandshake(t, path, time.Second); got != want {
		t.Fatalf("handshake = %+v, want %+v", got, want)
	}
}
