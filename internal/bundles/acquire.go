package bundles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"agent-vivy/internal/bundlecontract"
)

type AcquireRequest struct {
	OperationID    string
	Source         bundlecontract.SourceSpec
	StateRoot      string
	Limits         bundlecontract.Limits
	GitEnvironment GitEnvironment
}

func Acquire(ctx context.Context, in AcquireRequest) (Artifact, error) {
	var a Artifact
	if err := ctx.Err(); err != nil {
		return a, err
	}
	if !bundlecontract.ValidInstallID(in.OperationID) || in.StateRoot == "" {
		return a, bundlecontract.Invalid("acquisition")
	}
	l := in.Limits
	if l.ManifestBytes == 0 {
		l = bundlecontract.DefaultLimits()
	}
	base := filepath.Join(in.StateRoot, "content-bundles")
	staging := filepath.Join(base, "staging")
	objects := filepath.Join(base, "objects")
	for _, p := range []string{staging, objects} {
		if err := os.MkdirAll(p, 0700); err != nil {
			return a, err
		}
	}
	stage := filepath.Join(staging, in.OperationID)
	if err := os.Mkdir(stage, 0700); err != nil {
		return a, artifactError("revision_conflict", "operation_id", err)
	}
	defer os.RemoveAll(stage)
	source := in.Source.Location
	lock := bundlecontract.SourceLock{Kind: in.Source.Kind, ValidatorVersion: bundlecontract.APIVersion}
	switch in.Source.Kind {
	case "local":
		if in.Source.Ref != "" {
			return a, bundlecontract.Invalid("source_ref")
		}
		lock.Origin = filepath.Base(filepath.Clean(source))
	case "git":
		var e error
		source, lock, e = acquireGit(ctx, in.Source, stage, in.GitEnvironment)
		if e != nil {
			return a, e
		}
	default:
		return a, artifactError("unsupported_format", "source_kind", nil)
	}
	info, err := os.Lstat(source)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return a, artifactError("invalid_manifest", "source_directory", err)
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return a, err
	}
	defer root.Close()
	raw, err := readRegular(ctx, root, bundlecontract.ManifestFile, l.ManifestBytes)
	if os.IsNotExist(err) {
		return a, artifactError("unsupported_format", "native_manifest", err)
	}
	if err != nil {
		return a, err
	}
	m, err := bundlecontract.DecodeManifest(raw)
	if err != nil {
		return a, err
	}
	if err := bundlecontract.ValidateManifest(m, l); err != nil {
		return a, err
	}
	obj := filepath.Join(stage, "object")
	if err := os.MkdirAll(filepath.Join(obj, "files"), 0700); err != nil {
		return a, err
	}
	var inventory []bundlecontract.InventoryEntry
	var total int64
	for _, p := range bundlecontract.DeclaredPaths(m) {
		b, e := readRegular(ctx, root, p, l.ResourceBytes)
		if e != nil {
			return a, e
		}
		total += int64(len(b))
		if total > l.TotalBytes {
			return a, bundlecontract.Invalid("total_bytes")
		}
		dest := filepath.Join(obj, "files", filepath.FromSlash(p))
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return a, e
		}
		if e = writeSynced(dest, b); e != nil {
			return a, e
		}
		h := sha256.Sum256(b)
		inventory = append(inventory, bundlecontract.InventoryEntry{Path: p, Size: int64(len(b)), SHA256: hex.EncodeToString(h[:])})
	}
	digest, err := bundlecontract.ArtifactDigest(m, inventory)
	if err != nil {
		return a, err
	}
	a = Artifact{manifest: m, digest: digest, inventory: inventory, source: lock, directory: obj}
	if err := validateContent(ctx, a, l); err != nil {
		return Artifact{}, err
	}
	canonical, err := bundlecontract.CanonicalManifest(m)
	if err != nil {
		return Artifact{}, err
	}
	ib, err := bundlecontract.CanonicalJSON(inventory)
	if err != nil {
		return Artifact{}, err
	}
	for p, b := range map[string][]byte{"canonical-manifest.json": canonical, "inventory.json": ib} {
		if err := writeSynced(filepath.Join(obj, p), b); err != nil {
			return Artifact{}, err
		}
	}
	if err := syncTreeDirectories(obj); err != nil {
		return Artifact{}, err
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	target := filepath.Join(objects, digest)
	if _, err := os.Lstat(target); err == nil {
		x, e := OpenArtifact(ctx, in.StateRoot, digest)
		if e != nil {
			return Artifact{}, e
		}
		x.source = lock
		if in.Source.Kind == "local" {
			x.source.AcquisitionDigest = digest
		}
		return x, nil
	}
	if err := promoteDirectory(obj, target); err != nil {
		x, e := OpenArtifact(ctx, in.StateRoot, digest)
		if e != nil {
			return Artifact{}, err
		}
		x.source = lock
		return x, nil
	}
	if err := syncDirectory(objects); err != nil {
		return Artifact{}, err
	}
	a.directory = target
	if in.Source.Kind == "local" {
		a.source.AcquisitionDigest = digest
	}
	return a, nil
}
func writeSynced(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func syncTreeDirectories(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncDirectory(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}
