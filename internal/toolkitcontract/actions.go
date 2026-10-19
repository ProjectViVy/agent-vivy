// Package toolkitcontract contains the private, owner-scoped management
// facade for Vivy's native Toolkits control actions. It is not an SDK Port:
// only the kernel creates ActionHost values for the sealed vivy/toolkits T1
// owner.
package toolkitcontract

import (
	"context"
	"errors"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/storage"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	CodeInvalidInput        = "invalid_input"
	CodeNotFound            = "not_found"
	CodeConflict            = "revision_conflict"
	CodeUnavailable         = "owner_unavailable"
	CodeBusy                = "operation_pending"
	CodeBlocked             = "operation_blocked"
	CodeMaskInUse           = "mask_in_use"
	CodeConsentRequired     = "consent_required"
	CodeAuthorizationDenied = "authorization_denied"
	CodeCancelled           = "cancelled"
)

var allowedErrorCodes = map[string]struct{}{
	CodeInvalidInput: {}, CodeNotFound: {}, CodeConflict: {}, CodeUnavailable: {}, CodeBusy: {},
	CodeBlocked: {}, CodeMaskInUse: {}, CodeConsentRequired: {}, CodeAuthorizationDenied: {}, CodeCancelled: {},
}

type Error struct {
	Code  string
	cause error
}

func (e *Error) Error() string {
	if e == nil {
		return "toolkits: <nil>"
	}
	code := e.Code
	if _, ok := allowedErrorCodes[code]; !ok {
		code = CodeInvalidInput
	}
	return "toolkits: " + code
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
func NewError(code string, cause error) *Error {
	if _, ok := allowedErrorCodes[code]; !ok {
		code = CodeInvalidInput
	}
	return &Error{Code: code, cause: cause}
}
func IsErrorCode(code string) bool { _, ok := allowedErrorCodes[code]; return ok }
func IsCode(err error, code string) bool {
	var value *Error
	return errors.As(err, &value) && value != nil && value.Code == code
}

type Capabilities struct {
	Owners map[bundlecontract.ComponentKind]bool `json:"owners"`
}

// Manager is the smallest kernel-private manager surface used by the
// authenticated action host. The action provider never sees storage, owner
// replacement or artifact filesystem capabilities.
type Manager interface {
	ListInstallations(context.Context) (storage.BundleState, error)
	InspectInstallation(context.Context, string) (bundles.InstallationInspection, error)
	ReadComponent(context.Context, string, bundlecontract.ComponentKind, string, string, int, int) (bundles.ComponentPage, error)
	GetOperation(context.Context, string) (bundlecontract.Operation, error)
	Preview(context.Context, bundles.PreviewRequest) (bundles.PreparedSource, error)
	Install(context.Context, bundles.InstallRequest) (bundlecontract.Operation, error)
	Prepare(context.Context, bundles.PrepareRequest) (bundles.PreparedCatalog, error)
	PreparedCandidate(string) (string, bundlecontract.CatalogIdentity, bool)
	Publish(context.Context, string) (bundlecontract.Operation, error)
	CancelOperation(context.Context, string, int64) (bundlecontract.Operation, error)
	CancelPreview(string) bool
	SetExecutableConsent(context.Context, bundles.ExecutableConsentRequest) (bundlecontract.Operation, error)
	SetBindings(context.Context, bundles.BindingUpdateRequest) (bundlecontract.Operation, error)
}

// ActionHost is implemented by the private actionhost package. Every method
// scopes the provider context to the authenticated invocation before it calls
// Manager, and human-only actions derive their subject from trusted identity.
type ActionHost interface {
	controlaction.Host
	Capabilities() Capabilities
	HumanSubject() (string, error)
	RunBound() bool
	ListInstallations(context.Context) (storage.BundleState, error)
	InspectInstallation(context.Context, string) (bundles.InstallationInspection, error)
	ReadComponent(context.Context, string, bundlecontract.ComponentKind, string, string, int, int) (bundles.ComponentPage, error)
	GetOperation(context.Context, string) (bundlecontract.Operation, error)
	Preview(context.Context, bundles.PreviewRequest) (bundles.PreparedSource, error)
	Install(context.Context, bundles.InstallRequest) (bundlecontract.Operation, error)
	Prepare(context.Context, bundles.PrepareRequest) (bundles.PreparedCatalog, error)
	PreparedCandidate(string) (string, bundlecontract.CatalogIdentity, bool)
	Publish(context.Context, string) (bundlecontract.Operation, error)
	CancelOperation(context.Context, string, int64) (bundlecontract.Operation, error)
	CancelPreview(string) bool
	SetExecutableConsent(context.Context, bundles.ExecutableConsentRequest) (bundlecontract.Operation, error)
	SetBindings(context.Context, bundles.BindingUpdateRequest) (bundlecontract.Operation, error)
}
