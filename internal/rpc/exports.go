package rpc

import (
	"agent-vivy/internal/exportfile"
	"context"
	"encoding/base64"
	"errors"
	"os"
)

// exportsReadParams selects a write-once artifact by its opaque owner basename.
type exportsReadParams struct {
	Name string `json:"name"`
}

// exportsRead returns one complete bounded artifact from the configured root.
// Control connection authorization remains authoritative; a name grants no access.
func (h *controlHandler) exportsRead(ctx context.Context, request Request) (any, *Error) {
	if h.deps.ExportsDir == "" {
		return nil, &Error{Code: MethodNotFound, Message: "exports directory is not configured"}
	}
	var params exportsReadParams
	if err := decodeStrictJSON(request.Params, &params); err != nil {
		return nil, err
	}
	if err := validateExportFileName(params.Name); err != nil {
		return nil, err
	}
	body, err := exportfile.Read(ctx, h.deps.ExportsDir, params.Name)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist), errors.Is(err, exportfile.ErrUnavailable):
			return nil, &Error{Code: CodeNotFound, Message: "export is unavailable; create it again"}
		case errors.Is(err, exportfile.ErrTooLarge):
			return nil, &Error{Code: InvalidParams, Message: err.Error()}
		default:
			return nil, internalError(err)
		}
	}
	return map[string]any{"name": params.Name, "size": int64(len(body)), "data_base64": base64.StdEncoding.EncodeToString(body)}, nil
}

// validateExportFileName rejects foreign paths before the filesystem is touched.
func validateExportFileName(name string) *Error {
	if !exportfile.ValidName(name) {
		return &Error{Code: InvalidParams, Message: "name must be a file name inside the exports directory"}
	}
	return nil
}
