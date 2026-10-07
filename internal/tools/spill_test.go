package tools

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpillWriterStaysInMemoryBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	var inner strings.Builder
	w := newSpillWriter(&inner, dir, SpillFileName("call_1", "stdout"), 16)
	if _, err := w.Write([]byte("short")); err != nil {
		t.Fatal(err)
	}
	path, total, spilled := w.finish(true)
	if spilled || total != 5 || path != "" {
		t.Fatalf("finish = (%q, %d, %v), want no spill", path, total, spilled)
	}
	if inner.String() != "short" {
		t.Fatalf("inner = %q", inner.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("spill dir = %v, %v; want empty", entries, err)
	}
}

func TestSpillWriterSpillsFullStreamIncludingHead(t *testing.T) {
	dir := t.TempDir()
	var inner strings.Builder
	w := newSpillWriter(&inner, dir, SpillFileName("call;2", "stdout"), 8)
	for _, chunk := range []string{"head-", "body-", "tail"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	path, total, spilled := w.finish(true)
	if !spilled || total != 14 {
		t.Fatalf("finish = (%q, %d, %v), want spill total 14", path, total, spilled)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "head-body-tail" {
		t.Fatalf("spill file = %q, want full stream", data)
	}
	if filepath.Base(path) != "call_2.stdout.txt" {
		t.Fatalf("sanitized name = %q", filepath.Base(path))
	}
}

func TestJobRegistrySpillsOversizedOutput(t *testing.T) {
	dir := t.TempDir()
	registry := NewJobRegistry()
	const want = 100_000
	result, err := registry.RunForeground(context.Background(), JobSpec{
		Display: "big", SpillDir: dir, SpillID: "call_big", SpillBytes: 4 << 10,
		Run: func(ctx context.Context, stdout, _ io.Writer) error {
			_, err := stdout.Write([]byte(strings.Repeat("x", want)))
			return err
		},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.StdoutTrunc || result.StdoutSpillPath == "" || result.StdoutTotalBytes != want {
		t.Fatalf("result spill = %+v", result)
	}
	if len(result.Stdout) >= want {
		t.Fatalf("inline stdout = %d bytes, want bounded tail", len(result.Stdout))
	}
	data, err := os.ReadFile(result.StdoutSpillPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != want {
		t.Fatalf("spill file = %d bytes, want %d", len(data), want)
	}
	if filepath.Dir(result.StdoutSpillPath) != dir || filepath.Base(result.StdoutSpillPath) != "call_big.stdout.txt" {
		t.Fatalf("spill path = %q", result.StdoutSpillPath)
	}
}

func TestJobRegistrySmallOutputLeavesNoSpillFile(t *testing.T) {
	dir := t.TempDir()
	registry := NewJobRegistry()
	result, err := registry.RunForeground(context.Background(), JobSpec{
		Display: "small", SpillDir: dir, SpillID: "call_small",
		Run: func(ctx context.Context, stdout, _ io.Writer) error {
			_, err := stdout.Write([]byte("tiny"))
			return err
		},
	}, 10*time.Second)
	if err != nil || result.Stdout != "tiny" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if result.StdoutSpillPath != "" || result.StdoutTotalBytes != 0 {
		t.Fatalf("unexpected spill fields = %+v", result)
	}
	if _, err := os.Stat(dir); err == nil {
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatalf("spill dir has %v entries", entries)
		}
	}
}
