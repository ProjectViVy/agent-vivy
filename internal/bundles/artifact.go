package bundles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
)

type Capabilities struct {
	Owners map[bundlecontract.ComponentKind]bool
	Limits bundlecontract.Limits
}
type Artifact struct {
	manifest  bundlecontract.Manifest
	digest    string
	inventory []bundlecontract.InventoryEntry
	source    bundlecontract.SourceLock
	directory string
}

func (a Artifact) Manifest() bundlecontract.Manifest { return bundlecontract.CloneManifest(a.manifest) }
func (a Artifact) ReleaseDigest() string             { return a.digest }
func (a Artifact) Inventory() []bundlecontract.InventoryEntry {
	return append([]bundlecontract.InventoryEntry{}, a.inventory...)
}
func (a Artifact) SourceLock() bundlecontract.SourceLock { return a.source }
func artifactError(code, field string, cause error) error {
	return &bundlecontract.Error{Code: code, Field: field, Cause: cause}
}

func openRegular(root *os.Root, p string) (*os.File, error) {
	if err := bundlecontract.ValidateRelativePath(p); err != nil {
		return nil, err
	}
	parts := strings.Split(p, "/")
	for i := range parts {
		name := strings.Join(parts[:i+1], "/")
		s, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if s.Mode()&os.ModeSymlink != 0 {
			return nil, artifactError("invalid_manifest", "link", nil)
		}
		if i < len(parts)-1 && !s.IsDir() {
			return nil, artifactError("invalid_manifest", "directory", nil)
		}
	}
	f, err := root.OpenFile(p, os.O_RDONLY|noFollowFlag, 0)
	if err != nil {
		return nil, err
	}
	s, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !s.Mode().IsRegular() {
		f.Close()
		return nil, artifactError("invalid_manifest", "regular_file", nil)
	}
	if err := singleLink(f, s); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func readRegular(ctx context.Context, root *os.Root, p string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := openRegular(root, p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if s.Size() > limit {
		return nil, artifactError("invalid_manifest", "file_bytes", nil)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, artifactError("invalid_manifest", "file_bytes", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}
func (a Artifact) ReadDeclared(ctx context.Context, p string, maxBytes int64) ([]byte, error) {
	if err := bundlecontract.ValidateRelativePath(p); err != nil {
		return nil, err
	}
	var e *bundlecontract.InventoryEntry
	for i := range a.inventory {
		if a.inventory[i].Path == p {
			entry := a.inventory[i]
			e = &entry
			break
		}
	}
	if e == nil {
		return nil, artifactError("undeclared_resource", "path", nil)
	}
	if maxBytes < 0 || e.Size > maxBytes {
		return nil, artifactError("invalid_manifest", "read_bytes", nil)
	}
	r, err := os.OpenRoot(filepath.Join(a.directory, "files"))
	if err != nil {
		return nil, artifactError("artifact_unavailable", "files", err)
	}
	defer r.Close()
	b, err := readRegular(ctx, r, p, maxBytes)
	if err != nil {
		return nil, artifactError("artifact_unavailable", "file", err)
	}
	h := sha256.Sum256(b)
	if int64(len(b)) != e.Size || hex.EncodeToString(h[:]) != e.SHA256 {
		return nil, artifactError("artifact_corrupt", "file_digest", nil)
	}
	return b, nil
}
func OpenArtifact(ctx context.Context, stateRoot, digest string) (Artifact, error) {
	var a Artifact
	raw, err := hex.DecodeString(digest)
	if err != nil || len(raw) != sha256.Size || hex.EncodeToString(raw) != digest {
		return a, artifactError("artifact_corrupt", "digest", nil)
	}
	objectsPath := filepath.Join(stateRoot, "content-bundles", "objects")
	objects, err := os.OpenRoot(objectsPath)
	if err != nil {
		return a, artifactError("artifact_unavailable", "objects", err)
	}
	defer objects.Close()
	s, err := objects.Lstat(digest)
	if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 {
		return a, artifactError("artifact_unavailable", "object", err)
	}
	a.directory = filepath.Join(objectsPath, digest)
	root, err := objects.OpenRoot(digest)
	if err != nil {
		return a, err
	}
	defer root.Close()
	b, err := readRegular(ctx, root, "canonical-manifest.json", bundlecontract.DefaultLimits().ManifestBytes)
	if err != nil {
		return a, err
	}
	a.manifest, err = bundlecontract.DecodeManifest(b)
	if err != nil {
		return a, err
	}
	b, err = readRegular(ctx, root, "inventory.json", 2<<20)
	if err != nil {
		return a, err
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&a.inventory); err != nil {
		return a, artifactError("artifact_corrupt", "inventory", err)
	}
	a.digest, err = bundlecontract.ArtifactDigest(a.manifest, a.inventory)
	if err != nil || a.digest != digest {
		return Artifact{}, artifactError("artifact_corrupt", "release_digest", err)
	}
	for _, entry := range a.inventory {
		if _, err := a.ReadDeclared(ctx, entry.Path, bundlecontract.DefaultLimits().ResourceBytes); err != nil {
			return Artifact{}, err
		}
	}
	return a, nil
}
func ValidateArtifact(ctx context.Context, a Artifact, caps Capabilities) error {
	l := caps.Limits
	if l.ManifestBytes == 0 {
		l = bundlecontract.DefaultLimits()
	}
	if err := bundlecontract.ValidateManifest(a.manifest, l); err != nil {
		return err
	}
	for k, n := range map[bundlecontract.ComponentKind]int{bundlecontract.Skill: len(a.manifest.Components.Skills), bundlecontract.Mask: len(a.manifest.Components.Masks), bundlecontract.Rule: len(a.manifest.Components.Rules), bundlecontract.Hook: len(a.manifest.Components.Hooks), bundlecontract.MCP: len(a.manifest.Components.MCP)} {
		if n > 0 && !caps.Owners[k] {
			return artifactError("owner_unavailable", string(k), nil)
		}
	}
	return validateContent(ctx, a, l)
}
func validateContent(ctx context.Context, a Artifact, l bundlecontract.Limits) error {
	for _, s := range a.manifest.Components.Skills {
		b, e := a.ReadDeclared(ctx, s.Path, l.SkillBytes)
		if e != nil {
			return e
		}
		if _, e = DecodeNativeSkill(b); e != nil {
			return e
		}
	}
	for _, s := range a.manifest.Components.Masks {
		b, e := a.ReadDeclared(ctx, s.Path, l.MaskBytes)
		if e != nil {
			return e
		}
		if !utf8.Valid(b) || strings.TrimSpace(string(b)) == "" {
			return bundlecontract.Invalid("mask_body")
		}
	}
	for _, s := range a.manifest.Components.Rules {
		b, e := a.ReadDeclared(ctx, s.Path, l.RuleBytes)
		if e != nil {
			return e
		}
		if !utf8.Valid(b) {
			return bundlecontract.Invalid("rule_body")
		}
	}
	if p := a.manifest.Provenance; p != "" {
		b, e := a.ReadDeclared(ctx, p, l.ProvenanceBytes)
		if e != nil {
			return e
		}
		if !utf8.Valid(b) || !json.Valid(b) {
			return bundlecontract.Invalid("provenance")
		}
	}
	return nil
}
