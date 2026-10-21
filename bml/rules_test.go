package bml

import (
	"errors"
	"os"
	"sync"
	"testing"
)

func rulesConflict(t *testing.T, err error) {
	t.Helper()
	var e *HomeError
	if !errors.As(err, &e) || e.Code() != HomeCodeRevisionConflict {
		t.Fatalf("conflict = %v", err)
	}
}

func TestMemRulesExactCAS(t *testing.T) {
	h := NewHome(t.TempDir())
	current, err := h.ReadMemRules()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.WriteMemRules(current.Content); err != nil {
		t.Fatal(err)
	}
	_, err = h.CompareAndWriteMemRules("next", current.Content, false)
	rulesConflict(t, err)
	if err := os.WriteFile(h.MemRulesPath(), []byte("same\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = h.CompareAndWriteMemRules("next", "same\nbody", true)
	rulesConflict(t, err)
	updated, err := h.CompareAndWriteMemRules("next\r\nbody", "same\r\nbody", true)
	if err != nil || updated.Content != "next\nbody" {
		t.Fatalf("write = %+v, %v", updated, err)
	}
	_, err = h.CompareAndWriteMemRules("lost", "next\nBody", true)
	rulesConflict(t, err)
}

func TestMemRulesConcurrentCASHasOneWinner(t *testing.T) {
	h := NewHome(t.TempDir())
	current, err := h.ReadMemRules()
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, body := range []string{"writer one", "writer two"} {
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			<-start
			_, err := h.CompareAndWriteMemRules(body, current.Content, false)
			results <- err
		}(body)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			rulesConflict(t, err)
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestMemRulesCASFailurePreservesBody(t *testing.T) {
	h := NewHome(t.TempDir())
	if _, err := h.WriteMemRules("before"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.CompareAndWriteMemRules("  ", "before", true); err == nil {
		t.Fatal("blank write succeeded")
	}
	current, err := h.ReadMemRules()
	if err != nil || current.Content != "before" {
		t.Fatalf("after rejected write = %+v, %v", current, err)
	}
	if err := os.Remove(h.MemRulesPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(h.MemRulesPath(), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := h.CompareAndWriteMemRules("next", "before", true); err == nil {
		t.Fatal("read failure ignored")
	}
}
