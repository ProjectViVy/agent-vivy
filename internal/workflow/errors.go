package workflow

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSchemaInvalid         = errors.New("workflow schema invalid")
	ErrTopologyInvalid       = errors.New("workflow topology invalid")
	ErrCapabilityUnavailable = errors.New("workflow capability unavailable")
	ErrBudgetInvalid         = errors.New("workflow budget invalid")
	ErrAuthorityDenied       = errors.New("workflow authority denied")
	ErrExecutionUnavailable  = errors.New("workflow execution unavailable")
)

type Diagnostic struct {
	Check   string `json:"check"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationError struct {
	Diagnostics []Diagnostic
	causes      []error
}

func (err *ValidationError) Error() string {
	if err == nil {
		return ""
	}
	parts := make([]string, 0, len(err.Diagnostics))
	for _, diagnostic := range err.Diagnostics {
		if diagnostic.Path == "" {
			parts = append(parts, diagnostic.Message)
		} else {
			parts = append(parts, diagnostic.Path+": "+diagnostic.Message)
		}
	}
	if len(parts) == 0 {
		return "workflow definition is invalid"
	}
	return "workflow definition is invalid: " + strings.Join(parts, "; ")
}

func (err *ValidationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return errors.Join(err.causes...)
}

func validationError(diagnostics []Diagnostic, causes ...error) error {
	if len(diagnostics) == 0 {
		return nil
	}
	return &ValidationError{Diagnostics: append([]Diagnostic(nil), diagnostics...), causes: append([]error(nil), causes...)}
}

func diagnostic(check, path, message string) Diagnostic {
	return Diagnostic{Check: check, Path: path, Message: message}
}

func invalidDefinition(message string) error {
	return fmt.Errorf("workflow definition: %s", message)
}
