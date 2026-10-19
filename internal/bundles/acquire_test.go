package bundles

import (
	"agent-vivy/internal/bundlecontract"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const acquireID = "01234567-89ab-4def-8123-456789abcdef"

func sourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "plan"), 0700); err != nil {
		t.Fatal(err)
	}
	for p, b := range map[string]string{"vivy-bundle.json": `{"apiVersion":"vivy.bundle/v1","id":"example/experts","version":"1","name":"Experts","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]},"resources":["template.md"]}`, "skills/plan/SKILL.md": "---\ndescription: Plan\n---\nPlan work\n", "template.md": "template", "excluded.sh": "touch SHOULD-NOT-EXIST"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func TestAcquireNativeDirectory(t *testing.T) {
	source := sourceFixture(t)
	state := t.TempDir()
	a, err := Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: state, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Inventory()) != 2 || a.SourceLock().Kind != "local" {
		t.Fatal(a.Inventory())
	}
	b, err := a.ReadDeclared(context.Background(), "template.md", 1024)
	if err != nil || string(b) != "template" {
		t.Fatal(string(b), err)
	}
	if _, err := a.ReadDeclared(context.Background(), "excluded.sh", 1024); err == nil {
		t.Fatal("unlisted file exposed")
	}
	if _, err := a.ReadDeclared(context.Background(), "../template.md", 1024); err == nil {
		t.Fatal("escape exposed")
	}
	m := a.Manifest()
	m.Resources[0] = "changed"
	if a.Manifest().Resources[0] != "template.md" {
		t.Fatal("mutable manifest")
	}
	if err := os.WriteFile(filepath.Join(source, "template.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err = a.ReadDeclared(context.Background(), "template.md", 1024)
	if err != nil || string(b) != "template" {
		t.Fatal("source mutation changed artifact")
	}
	b, err = a.ReadDeclared(context.Background(), "template.md", 1)
	if err == nil {
		t.Fatal("read limit ignored")
	}
	_, err = OpenArtifact(context.Background(), state, a.ReleaseDigest())
	if err != nil {
		t.Fatal(err)
	}
}
func TestAcquireRejectsEscapes(t *testing.T) {
	for _, mode := range []string{"symlink", "hardlink", "parent-link"} {
		t.Run(mode, func(t *testing.T) {
			source := sourceFixture(t)
			p := filepath.Join(source, "template.md")
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			var err error
			switch mode {
			case "symlink":
				err = os.Symlink(target, p)
			case "hardlink":
				err = os.Link(target, p)
			case "parent-link":
				err = os.Symlink(target, p)
			}
			if err != nil {
				t.Skip(err)
			}
			_, err = Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: t.TempDir(), Limits: bundlecontract.DefaultLimits()})
			if err == nil {
				t.Fatal("link accepted")
			}
		})
	}
	source := sourceFixture(t)
	l := bundlecontract.DefaultLimits()
	l.TotalBytes = 1
	_, err := Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: t.TempDir(), Limits: l})
	if err == nil {
		t.Fatal("total size ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Acquire(ctx, AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: t.TempDir(), Limits: bundlecontract.DefaultLimits()})
	if err == nil {
		t.Fatal("cancel ignored")
	}
}
func TestObjectRecoveryRejectsTampering(t *testing.T) {
	state := t.TempDir()
	a, err := Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: sourceFixture(t)}, StateRoot: state, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(state, "content-bundles", "objects", a.ReleaseDigest(), "files", "template.md")
	if err := os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ReadDeclared(context.Background(), "template.md", 1024); err == nil {
		t.Fatal("tampered read accepted")
	}
	if _, err := OpenArtifact(context.Background(), state, a.ReleaseDigest()); err == nil {
		t.Fatal("tampered object recovered")
	}
}

func TestGitAcquisitionHasNoExecution(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip(err)
	}
	remote := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command(git, args...)
		c.Dir = remote
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if out, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s %v", args, out, e)
		}
	}
	if out, e := exec.Command(git, "init", remote).CombinedOutput(); e != nil {
		t.Fatal(string(out), e)
	}
	if err := os.MkdirAll(filepath.Join(remote, "skills", "plan"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/experts","version":"1","name":"Experts","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]},"resources":["template.md"]}`
	for p, b := range map[string]string{"vivy-bundle.json": manifest, "skills/plan/SKILL.md": "---\ndescription: Plan\n---\nPlan work\n", "template.md": "template", "install.sh": "touch SHOULD-NOT-EXIST"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(remote, p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(remote, p), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "add", ".")
	run("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "fixture")
	if err := os.WriteFile(filepath.Join(remote, ".git", "hooks", "post-checkout"), []byte("#!/bin/sh\ntouch SHOULD-NOT-EXIST\n"), 0700); err != nil {
		t.Fatal(err)
	}
	commitOut, e := exec.Command(git, "-C", remote, "rev-parse", "HEAD").Output()
	if e != nil {
		t.Fatal(e)
	}
	commit := strings.TrimSpace(string(commitOut))
	a, e := Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "git", Location: remote, Ref: "HEAD"}, StateRoot: t.TempDir(), Limits: bundlecontract.DefaultLimits()})
	if e != nil {
		t.Fatal(e)
	}
	if a.SourceLock().ResolvedCommit != commit || len(a.Inventory()) != 2 {
		t.Fatal(a.SourceLock(), a.Inventory())
	}
	if _, e := os.Stat(filepath.Join(remote, "SHOULD-NOT-EXIST")); !os.IsNotExist(e) {
		t.Fatal("repository hook executed", e)
	}
}

func TestAcquireReportsMissingOwnerAndRejectsCredentialOrigin(t *testing.T) {
	state := t.TempDir()
	a, err := Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "local", Location: sourceFixture(t)}, StateRoot: state, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateArtifact(context.Background(), a, Capabilities{Owners: map[bundlecontract.ComponentKind]bool{}, Limits: bundlecontract.DefaultLimits()})
	var e *bundlecontract.Error
	if !errors.As(err, &e) || e.Code != "owner_unavailable" {
		t.Fatal(err)
	}
	_, err = Acquire(context.Background(), AcquireRequest{OperationID: acquireID, Source: bundlecontract.SourceSpec{Kind: "git", Location: "https://user:secret@example.invalid/repo", Ref: "main"}, StateRoot: t.TempDir(), Limits: bundlecontract.DefaultLimits()})
	if !errors.As(err, &e) || e.Code != "invalid_manifest" {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("credential leaked", err)
	}
}
