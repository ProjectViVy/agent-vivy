package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// exportsReadMaxBytes bounds one exports/read transfer. Session HTML and
// bug-report markdown are far below this; anything larger answers an error
// rather than a partial file.
const exportsReadMaxBytes = 32 << 20

// exportsReadParams addresses one artifact by basename inside the exports
// directory. expected_digest binds the read to the digest the caller was
// shown at export time; a file that changed answers "changed".
type exportsReadParams struct {
	Name           string `json:"name"`
	ExpectedDigest string `json:"expected_digest,omitempty"`
}

// exportsRead serves exports/read (VCP C3): a digest-bound read of one file
// inside the configured exports directory — the GUI's verified-download
// path for session exports and /bug bundles. The directory is a controlled
// artifact root: names must be bare file names, so no path leaves the dir.
func (h *controlHandler) exportsRead(ctx context.Context, request Request) (any, *Error) {
	if h.deps.ExportsDir == "" {
		return nil, &Error{Code: MethodNotFound, Message: "exports directory is not configured"}
	}
	var params exportsReadParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if rpcErr := validateExportFileName(params.Name); rpcErr != nil {
		return nil, rpcErr
	}
	path := filepath.Join(h.deps.ExportsDir, params.Name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &Error{Code: CodeNotFound, Message: "export not found"}
		}
		return nil, internalError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, &Error{Code: CodeNotFound, Message: "export not found"}
	}
	if info.Size() > exportsReadMaxBytes {
		return nil, &Error{Code: InvalidParams, Message: fmt.Sprintf("export exceeds %d bytes", exportsReadMaxBytes)}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, internalError(err)
	}
	digest := sha256.Sum256(body)
	digestHex := hex.EncodeToString(digest[:])
	if params.ExpectedDigest != "" && !strings.EqualFold(params.ExpectedDigest, digestHex) {
		return nil, &Error{Code: CodeNotFound, Message: "export changed since the digest was bound"}
	}
	return map[string]any{
		"name":        params.Name,
		"digest":      digestHex,
		"size":        info.Size(),
		"data_base64": base64.StdEncoding.EncodeToString(body),
	}, nil
}

// validateExportFileName accepts only bare artifact basenames from the
// export-writer charset: letters, digits, dot, dash, underscore. Anything
// with a separator or traversal is rejected before the filesystem is
// touched.
func validateExportFileName(name string) *Error {
	if name == "" {
		return &Error{Code: InvalidParams, Message: "name is required"}
	}
	if name == "." || name == ".." || name != filepath.Base(name) || filepath.Clean(name) != name {
		return &Error{Code: InvalidParams, Message: "name must be a file name inside the exports directory"}
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return &Error{Code: InvalidParams, Message: "name must be a file name inside the exports directory"}
		}
	}
	return nil
}
