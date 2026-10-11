package toolkits

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/toolkitcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	OwnerID         = "vivy/toolkits"
	MaxActionInput  = 256 << 10
	MaxActionOutput = 512 << 10
)

const (
	ActionList             = "vivy.bundles.list"
	ActionGet              = "vivy.bundles.get"
	ActionInspect          = "vivy.bundles.inspect"
	ActionPreview          = "vivy.bundles.preview"
	ActionPreviewGit       = "vivy.bundles.preview-git"
	ActionInstall          = "vivy.bundles.install"
	ActionPrepareUpdate    = "vivy.bundles.prepare-update"
	ActionPrepareUpdateGit = "vivy.bundles.prepare-update-git"
	ActionEnable           = "vivy.bundles.enable"
	ActionDisable          = "vivy.bundles.disable"
	ActionUninstall        = "vivy.bundles.uninstall"
	ActionRollback         = "vivy.bundles.rollback"
	ActionOperationGet     = "vivy.bundles.operation.get"
	ActionOperationPublish = "vivy.bundles.operation.publish"
	ActionOperationCancel  = "vivy.bundles.operation.cancel"
	ActionComponentGet     = "vivy.bundles.component.get"
	ActionHookArm          = "vivy.bundles.hook.arm"
	ActionHookDisarm       = "vivy.bundles.hook.disarm"
	ActionMCPAuthorize     = "vivy.bundles.mcp.authorize"
	ActionMCPRevoke        = "vivy.bundles.mcp.revoke"
	ActionBindingsSet      = "vivy.bundles.bindings.set"
	ActionSourceRebind     = "vivy.bundles.source.rebind"
	ActionSourceRebindGit  = "vivy.bundles.source.rebind-git"
)

var resultSchema = json.RawMessage(`{"type":"object","additionalProperties":true}`)
var catalogProperties = `"expected_catalog":{"type":"object","required":["epoch","config_revision","effective_digest","generation_id"],"properties":{"epoch":{"type":"integer","minimum":0},"config_revision":{"type":"integer","minimum":0},"effective_digest":{"type":"string"},"generation_id":{"type":"string"}},"additionalProperties":false}`

func objectSchema(required, properties string) json.RawMessage {
	return json.RawMessage(`{"type":"object","required":[` + required + `],"properties":{` + properties + `},"additionalProperties":false}`)
}

type providerSpec struct {
	id, description string
	effect          controlaction.Effect
	input           json.RawMessage
	invoke          func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error)
}
type toolkitAction struct {
	definition controlaction.Definition
	invoke     func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error)
}

func (a toolkitAction) Definition() controlaction.Definition { return a.definition }
func (a toolkitAction) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	private, ok := host.(toolkitcontract.ActionHost)
	if !ok || private == nil {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeUnavailable, nil)
	}
	if len(input) > MaxActionInput {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
	}
	return a.invoke(ctx, private, append(json.RawMessage(nil), input...))
}

func ActionProviders() []controlaction.Provider {
	preview := objectSchema(`"operation_id","expected_catalog","location"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"location":{"type":"string","minLength":1},"ref":{"type":"string"}`)
	mutation := objectSchema(`"operation_id","expected_catalog","install_id","expected_install_revision"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"install_id":{"type":"string"},"expected_install_revision":{"type":"integer","minimum":1}`)
	update := objectSchema(`"operation_id","preview_operation_id","expected_catalog","install_id","expected_install_revision","location"`, `"operation_id":{"type":"string"},"preview_operation_id":{"type":"string"},`+catalogProperties+`,"install_id":{"type":"string"},"expected_install_revision":{"type":"integer","minimum":1},"location":{"type":"string","minLength":1},"ref":{"type":"string"}`)
	rollback := objectSchema(`"operation_id","expected_catalog","install_id","expected_install_revision","release_digest"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"install_id":{"type":"string"},"expected_install_revision":{"type":"integer","minimum":1},"release_digest":{"type":"string"}`)
	consent := objectSchema(`"operation_id","expected_catalog","install_id","component_id","release_digest","declaration_digest","expected_consent_revision"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"install_id":{"type":"string"},"component_id":{"type":"string"},"release_digest":{"type":"string"},"declaration_digest":{"type":"string"},"expected_consent_revision":{"type":"integer","minimum":0}`)
	publish := objectSchema(`"operation_id","candidate_digest","expected_catalog"`, `"operation_id":{"type":"string"},"candidate_digest":{"type":"string"},`+catalogProperties)
	items := []providerSpec{
		{ActionList, "List installed Toolkits and compiled owner capabilities", controlaction.EffectRead, objectSchema(``, `"after_id":{"type":"string"},"limit":{"type":"integer","minimum":0,"maximum":100},"state":{"type":"string"}`), invokeList},
		{ActionGet, "Get Toolkit installation metadata", controlaction.EffectRead, objectSchema(`"install_id"`, `"install_id":{"type":"string"}`), invokeInspect},
		{ActionInspect, "Inspect a native Toolkit manifest and consent state", controlaction.EffectRead, objectSchema(`"install_id"`, `"install_id":{"type":"string"}`), invokeInspect},
		{ActionPreview, "Acquire and validate a local native Toolkit directory", controlaction.EffectExternalEffect, preview, invokePreview("local")},
		{ActionPreviewGit, "Acquire and validate a Git native Toolkit source", controlaction.EffectExternalEffect, preview, invokePreview("git")},
		{ActionInstall, "Install the exact reviewed Toolkit preview disabled", controlaction.EffectWrite, objectSchema(`"operation_id","expected_catalog","preview_operation_id","expected_release_digest"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"preview_operation_id":{"type":"string"},"expected_release_digest":{"type":"string"}`), invokeInstall},
		{ActionPrepareUpdate, "Prepare an update from the same local source", controlaction.EffectExternalEffect, update, invokePrepareSource("local", "update")},
		{ActionPrepareUpdateGit, "Prepare an update from the same Git origin", controlaction.EffectExternalEffect, update, invokePrepareSource("git", "update")},
		{ActionEnable, "Prepare to enable a disabled Toolkit", controlaction.EffectWrite, mutation, invokeMutation("enable")},
		{ActionDisable, "Prepare to disable an enabled Toolkit", controlaction.EffectWrite, mutation, invokeMutation("disable")},
		{ActionUninstall, "Prepare to uninstall a disabled Toolkit", controlaction.EffectWrite, mutation, invokeMutation("uninstall")},
		{ActionRollback, "Prepare rollback to a retained Toolkit release", controlaction.EffectWrite, rollback, invokeMutation("rollback")},
		{ActionOperationGet, "Read a durable Toolkit operation receipt", controlaction.EffectRead, objectSchema(`"operation_id"`, `"operation_id":{"type":"string"}`), invokeOperationGet},
		{ActionOperationPublish, "Publish the exact reviewed Toolkit candidate", controlaction.EffectWrite, publish, invokePublish},
		{ActionOperationCancel, "Cancel a pending Toolkit operation by revision", controlaction.EffectWrite, objectSchema(`"operation_id","expected_operation_revision"`, `"operation_id":{"type":"string"},"expected_operation_revision":{"type":"integer","minimum":1}`), invokeCancel},
		{ActionComponentGet, "Read a bounded page of one native component body", controlaction.EffectRead, objectSchema(`"install_id","kind","local_id","expected_release_digest"`, `"install_id":{"type":"string"},"kind":{"enum":["skill","mask","rule","hook","mcp"]},"local_id":{"type":"string"},"expected_release_digest":{"type":"string"},"offset":{"type":"integer","minimum":0},"max_bytes":{"type":"integer","minimum":0,"maximum":65536}`), invokeComponent},
		{ActionHookArm, "Arm one exact Hook declaration after human review", controlaction.EffectWrite, consent, invokeConsent(bundlecontract.Hook, true)},
		{ActionHookDisarm, "Revoke one exact Hook declaration", controlaction.EffectWrite, consent, invokeConsent(bundlecontract.Hook, false)},
		{ActionMCPAuthorize, "Authorize one exact MCP declaration after human review", controlaction.EffectWrite, consent, invokeConsent(bundlecontract.MCP, true)},
		{ActionMCPRevoke, "Revoke one exact MCP declaration", controlaction.EffectWrite, consent, invokeConsent(bundlecontract.MCP, false)},
		{ActionBindingsSet, "Set named Hook or MCP environment references without returning secret values", controlaction.EffectWrite, objectSchema(`"operation_id","expected_catalog","expected_config_revision","install_id","kind","component_id","bindings"`, `"operation_id":{"type":"string"},`+catalogProperties+`,"expected_config_revision":{"type":"integer","minimum":0},"install_id":{"type":"string"},"kind":{"enum":["hook","mcp"]},"component_id":{"type":"string"},"bindings":{"type":"array","items":{"type":"object","required":["name","reference"],"properties":{"name":{"type":"string"},"reference":{"type":"string"}},"additionalProperties":false}}`), invokeBindings},
		{ActionSourceRebind, "Review a local Toolkit source rebind", controlaction.EffectExternalEffect, update, invokeRebind("local")},
		{ActionSourceRebindGit, "Review a Git Toolkit source rebind", controlaction.EffectExternalEffect, update, invokeRebind("git")},
	}
	providers := make([]controlaction.Provider, 0, len(items))
	for _, item := range items {
		providers = append(providers, toolkitAction{definition: controlaction.Definition{ID: item.id, Owner: OwnerID, ModuleID: OwnerID, Description: item.description, Effect: item.effect, InputSchema: item.input, ResultSchema: resultSchema, MaxInputBytes: MaxActionInput, MaxOutputBytes: MaxActionOutput}, invoke: item.invoke})
	}
	return providers
}

type previewInput struct {
	OperationID string                         `json:"operation_id"`
	Expected    bundlecontract.CatalogIdentity `json:"expected_catalog"`
	Location    string                         `json:"location"`
	Ref         string                         `json:"ref"`
}
type installInput struct {
	OperationID           string                         `json:"operation_id"`
	Expected              bundlecontract.CatalogIdentity `json:"expected_catalog"`
	PreviewOperationID    string                         `json:"preview_operation_id"`
	ExpectedReleaseDigest string                         `json:"expected_release_digest"`
}
type mutationInput struct {
	OperationID             string                         `json:"operation_id"`
	Expected                bundlecontract.CatalogIdentity `json:"expected_catalog"`
	InstallID               string                         `json:"install_id"`
	ExpectedInstallRevision int64                          `json:"expected_install_revision"`
	ReleaseDigest           string                         `json:"release_digest"`
	PreviewOperationID      string                         `json:"preview_operation_id"`
	Location                string                         `json:"location"`
	Ref                     string                         `json:"ref"`
}
type publishInput struct {
	OperationID     string                         `json:"operation_id"`
	CandidateDigest string                         `json:"candidate_digest"`
	Expected        bundlecontract.CatalogIdentity `json:"expected_catalog"`
}
type cancelInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_operation_revision"`
}
type componentInput struct {
	InstallID     string                       `json:"install_id"`
	Kind          bundlecontract.ComponentKind `json:"kind"`
	LocalID       string                       `json:"local_id"`
	ReleaseDigest string                       `json:"expected_release_digest"`
	Offset        int                          `json:"offset"`
	MaxBytes      int                          `json:"max_bytes"`
}
type consentInput struct {
	OperationID       string                         `json:"operation_id"`
	Expected          bundlecontract.CatalogIdentity `json:"expected_catalog"`
	InstallID         string                         `json:"install_id"`
	ComponentID       string                         `json:"component_id"`
	ReleaseDigest     string                         `json:"release_digest"`
	DeclarationDigest string                         `json:"declaration_digest"`
	ExpectedRevision  int64                          `json:"expected_consent_revision"`
}
type bindingInput struct {
	OperationID            string                         `json:"operation_id"`
	Expected               bundlecontract.CatalogIdentity `json:"expected_catalog"`
	ExpectedConfigRevision int64                          `json:"expected_config_revision"`
	InstallID              string                         `json:"install_id"`
	Kind                   bundlecontract.ComponentKind   `json:"kind"`
	ComponentID            string                         `json:"component_id"`
	Bindings               []bundlecontract.BindingRef    `json:"bindings"`
}

func decode(raw []byte, dst any) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, err)
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
	}
	return nil
}
func rejectDuplicateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return errors.New("json depth")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case nil:
			return errors.New("json null")
		case json.Delim:
			switch value {
			case '{':
				keys := map[string]bool{}
				for d.More() {
					item, e := d.Token()
					if e != nil {
						return e
					}
					key, ok := item.(string)
					if !ok || keys[key] {
						return errors.New("json duplicate key")
					}
					keys[key] = true
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
				_, err = d.Token()
				return err
			case '[':
				for d.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
				_, err = d.Token()
				return err
			default:
				return errors.New("json delimiter")
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("json trailing data")
	}
	return nil
}
func marshal(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > MaxActionOutput {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeUnavailable, err)
	}
	return data, nil
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "not_found"), strings.Contains(text, "not found"):
		return toolkitcontract.NewError(toolkitcontract.CodeNotFound, err)
	case strings.Contains(text, "revision_conflict"), strings.Contains(text, "preview_stale"), strings.Contains(text, "stale"), strings.Contains(text, "conflict"):
		return toolkitcontract.NewError(toolkitcontract.CodeConflict, err)
	case strings.Contains(text, "mask_in_use"):
		return toolkitcontract.NewError(toolkitcontract.CodeMaskInUse, err)
	case strings.Contains(text, "catalog_update_pending"), strings.Contains(text, "publish_in_progress"):
		return toolkitcontract.NewError(toolkitcontract.CodeBusy, err)
	case strings.Contains(text, "cancel"):
		return toolkitcontract.NewError(toolkitcontract.CodeCancelled, err)
	default:
		return toolkitcontract.NewError(toolkitcontract.CodeUnavailable, err)
	}
}
func requireHuman(host toolkitcontract.ActionHost) error {
	if host.RunBound() {
		return toolkitcontract.NewError(toolkitcontract.CodeAuthorizationDenied, nil)
	}
	_, err := host.HumanSubject()
	return err
}
func validUUID(value string) bool { return bundlecontract.ValidInstallID(value) }

func invokeList(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		AfterID string `json:"after_id"`
		Limit   int    `json:"limit"`
		State   string `json:"state"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	state, err := host.ListInstallations(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	limit := in.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
	}
	items := make([]storage.BundleInstallation, 0, limit)
	started := in.AfterID == ""
	for _, item := range state.Installations {
		if !started {
			if item.InstallID == in.AfterID {
				started = true
			}
			continue
		}
		if in.State != "" && item.State != in.State {
			continue
		}
		items = append(items, item)
		if len(items) > limit {
			break
		}
	}
	next := ""
	if len(items) > limit {
		next = items[limit-1].InstallID
		items = items[:limit]
	}
	return marshal(struct {
		Items        []storage.BundleInstallation   `json:"items"`
		NextAfterID  string                         `json:"next_after_id"`
		Catalog      bundlecontract.CatalogIdentity `json:"catalog"`
		Capabilities toolkitcontract.Capabilities   `json:"capabilities"`
	}{items, next, state.Catalog, host.Capabilities()})
}
func invokeInspect(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		InstallID string `json:"install_id"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	value, err := host.InspectInstallation(ctx, in.InstallID)
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokePreview(kind string) func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
		var in previewInput
		if err := decode(raw, &in); err != nil {
			return nil, err
		}
		if err := requireHuman(host); err != nil {
			return nil, err
		}
		if !validUUID(in.OperationID) || strings.TrimSpace(in.Location) == "" {
			return nil, toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
		}
		value, err := host.Preview(ctx, bundles.PreviewRequest{OperationID: in.OperationID, Expected: in.Expected, Source: bundlecontract.SourceSpec{Kind: kind, Location: in.Location, Ref: in.Ref}})
		if err != nil {
			return nil, mapError(err)
		}
		return marshal(value)
	}
}
func invokeInstall(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in installInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if err := requireHuman(host); err != nil {
		return nil, err
	}
	value, err := host.Install(ctx, bundles.InstallRequest{OperationID: in.OperationID, Expected: in.Expected, PreviewOperationID: in.PreviewOperationID, ExpectedReleaseDigest: in.ExpectedReleaseDigest})
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokePrepareSource(kind, mutation string) func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
		var in mutationInput
		if err := decode(raw, &in); err != nil {
			return nil, err
		}
		if err := requireHuman(host); err != nil {
			return nil, err
		}
		preview, err := host.Preview(ctx, bundles.PreviewRequest{OperationID: in.PreviewOperationID, Expected: in.Expected, Source: bundlecontract.SourceSpec{Kind: kind, Location: in.Location, Ref: in.Ref}})
		if err != nil {
			return nil, mapError(err)
		}
		prepared, err := host.Prepare(ctx, bundles.PrepareRequest{OperationID: in.OperationID, Expected: in.Expected, Mutation: bundles.CatalogMutation{Kind: mutation, InstallID: in.InstallID, ReleaseDigest: preview.ReleaseDigest, PreviewOperationID: preview.OperationID, ExpectedRevision: in.ExpectedInstallRevision}})
		if err != nil {
			host.CancelPreview(preview.OperationID)
			return nil, mapError(err)
		}
		return marshal(preparedResult(prepared))
	}
}
func invokeMutation(kind string) func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
		var in mutationInput
		if err := decode(raw, &in); err != nil {
			return nil, err
		}
		if err := requireHuman(host); err != nil {
			return nil, err
		}
		prepared, err := host.Prepare(ctx, bundles.PrepareRequest{OperationID: in.OperationID, Expected: in.Expected, Mutation: bundles.CatalogMutation{Kind: kind, InstallID: in.InstallID, ReleaseDigest: in.ReleaseDigest, ExpectedRevision: in.ExpectedInstallRevision}})
		if err != nil {
			return nil, mapError(err)
		}
		return marshal(preparedResult(prepared))
	}
}
func preparedResult(prepared bundles.PreparedCatalog) any {
	operation := prepared.Operation
	if operation.ID == "" {
		operation = bundlecontract.Operation{ID: prepared.OperationID, Kind: "publish", State: "pending", Revision: 1, ExpectedEpoch: prepared.Expected.Epoch, Blockers: []bundlecontract.Blocker{}}
	}
	return struct {
		Operation       bundlecontract.Operation       `json:"operation"`
		CandidateDigest string                         `json:"candidate_digest"`
		Expected        bundlecontract.CatalogIdentity `json:"expected_catalog"`
		Releases        []bundlecontract.ReleaseRef    `json:"releases"`
	}{operation, prepared.CandidateDigest, prepared.Expected, prepared.Releases}
}
func invokeOperationGet(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		OperationID string `json:"operation_id"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	value, err := host.GetOperation(ctx, in.OperationID)
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokePublish(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in publishInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if err := requireHuman(host); err != nil {
		return nil, err
	}
	digest, expected, ok := host.PreparedCandidate(in.OperationID)
	if !ok || digest != in.CandidateDigest || expected != in.Expected {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeConflict, nil)
	}
	value, err := host.Publish(ctx, in.OperationID)
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokeCancel(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in cancelInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if err := requireHuman(host); err != nil {
		return nil, err
	}
	value, err := host.CancelOperation(ctx, in.OperationID, in.ExpectedRevision)
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokeComponent(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in componentInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	value, err := host.ReadComponent(ctx, in.InstallID, in.Kind, in.LocalID, in.ReleaseDigest, in.Offset, in.MaxBytes)
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokeConsent(kind bundlecontract.ComponentKind, allowed bool) func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
		var in consentInput
		if err := decode(raw, &in); err != nil {
			return nil, err
		}
		if err := requireHuman(host); err != nil {
			return nil, err
		}
		subject, err := host.HumanSubject()
		if err != nil {
			return nil, err
		}
		value, err := host.SetExecutableConsent(ctx, bundles.ExecutableConsentRequest{OperationID: in.OperationID, Expected: in.Expected, Ref: bundlecontract.ComponentRef{InstallID: in.InstallID, Kind: kind, LocalID: in.ComponentID}, ExpectedRevision: in.ExpectedRevision, ExpectedReleaseDigest: in.ReleaseDigest, ExpectedDeclarationDigest: in.DeclarationDigest, Subject: subject, Allowed: allowed})
		if err != nil {
			return nil, mapError(err)
		}
		return marshal(value)
	}
}
func invokeBindings(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in bindingInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if err := requireHuman(host); err != nil {
		return nil, err
	}
	if !validUUID(in.OperationID) || (in.Kind != bundlecontract.Hook && in.Kind != bundlecontract.MCP) || !bundlecontract.ValidLocalID(in.ComponentID) || in.ExpectedConfigRevision != in.Expected.ConfigRevision {
		return nil, toolkitcontract.NewError(toolkitcontract.CodeInvalidInput, nil)
	}
	subject, err := host.HumanSubject()
	if err != nil {
		return nil, err
	}
	value, err := host.SetBindings(ctx, bundles.BindingUpdateRequest{
		OperationID: in.OperationID, Expected: in.Expected, ExpectedConfigRevision: in.ExpectedConfigRevision,
		InstallID: in.InstallID, Kind: in.Kind, LocalID: in.ComponentID, Bindings: in.Bindings, Subject: subject,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return marshal(value)
}
func invokeRebind(kind string) func(context.Context, toolkitcontract.ActionHost, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, host toolkitcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
		var in mutationInput
		if err := decode(raw, &in); err != nil {
			return nil, err
		}
		if err := requireHuman(host); err != nil {
			return nil, err
		}
		preview, err := host.Preview(ctx, bundles.PreviewRequest{OperationID: in.PreviewOperationID, Expected: in.Expected, Source: bundlecontract.SourceSpec{Kind: kind, Location: in.Location, Ref: in.Ref}})
		if err != nil {
			return nil, mapError(err)
		}
		prepared, err := host.Prepare(ctx, bundles.PrepareRequest{OperationID: in.OperationID, Expected: in.Expected, Mutation: bundles.CatalogMutation{Kind: "rebind", InstallID: in.InstallID, ReleaseDigest: preview.ReleaseDigest, PreviewOperationID: preview.OperationID, ExpectedRevision: in.ExpectedInstallRevision}})
		if err != nil {
			host.CancelPreview(preview.OperationID)
			return nil, mapError(err)
		}
		return marshal(preparedResult(prepared))
	}
}
