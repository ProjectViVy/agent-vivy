// Package sourcehash computes the canonical content identity of a Module source tree.
package sourcehash

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

const generatedAssemblyPath = "generated/assembly/zz_default.go"

type sourceFile struct {
	path string
	rel  string
}

// Tree hashes a deterministic path/content stream. Text line endings are
// canonicalized to LF so Git checkout policy cannot change a Module identity.
// The declared digest is zeroed to avoid a circular content address.
func Tree(root, declaredDigest string) (string, error) {
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source tree contains symbolic link %s", path)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == generatedAssemblyPath {
			return nil
		}
		files = append(files, sourceFile{path: path, rel: rel})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	h := sha256.New()
	for _, file := range files {
		body, err := os.ReadFile(file.path)
		if err != nil {
			return "", err
		}
		if declaredDigest != "" {
			body = bytes.ReplaceAll(body, []byte(declaredDigest), bytes.Repeat([]byte{'0'}, sha256.Size*2))
		}
		if utf8.Valid(body) && bytes.IndexByte(body, 0) < 0 {
			body = bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
		}
		_, _ = io.WriteString(h, file.rel)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
