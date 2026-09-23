package masks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	ActionCatalogList   = "vivy.masks.catalog.list"
	ActionCatalogGet    = "vivy.masks.catalog.get"
	ActionCatalogCreate = "vivy.masks.catalog.create"
	ActionCatalogUpdate = "vivy.masks.catalog.update"
	ActionCatalogDelete = "vivy.masks.catalog.delete"
	ActionSelectionGet  = "vivy.masks.selection.get"
	ActionSelectionSet  = "vivy.masks.selection.set"
)

const (
	maxActionInput  = 32 << 10
	maxActionOutput = 256 << 10
)

type maskAction struct {
	definition controlaction.Definition
	invoke     func(context.Context, maskcontract.ActionHost, json.RawMessage) (json.RawMessage, error)
}

func (a maskAction) Definition() controlaction.Definition { return a.definition }

func (a maskAction) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	privateHost, ok := host.(maskcontract.ActionHost)
	if !ok || privateHost == nil {
		return nil, maskcontract.NewError(maskcontract.CodeAuthorizationDenied, errors.New("mask action facade unavailable"))
	}
	if len(input) > maxActionInput {
		return nil, maskcontract.NewError(maskcontract.CodeInvalidMask, errors.New("mask action input exceeds limit"))
	}
	return a.invoke(ctx, privateHost, append(json.RawMessage(nil), input...))
}

// ActionProviders returns the closed action inventory owned by vivy/masks.
// Assembly binding remains compiler-owned; this value is deliberately just a
// typed provider set for selected-generation fixtures and the future binder.
func ActionProviders() []controlaction.Provider {
	return []controlaction.Provider{
		maskAction{definition: actionDefinition(ActionCatalogList, "List mask metadata", controlaction.EffectRead, listInputSchema, pageSchema), invoke: invokeList},
		maskAction{definition: actionDefinition(ActionCatalogGet, "Get one mask definition", controlaction.EffectRead, getInputSchema, definitionSchema), invoke: invokeGet},
		maskAction{definition: actionDefinition(ActionCatalogCreate, "Create a custom mask", controlaction.EffectWrite, createInputSchema, definitionSchema), invoke: invokeCreate},
		maskAction{definition: actionDefinition(ActionCatalogUpdate, "Update a custom mask", controlaction.EffectWrite, updateInputSchema, definitionSchema), invoke: invokeUpdate},
		maskAction{definition: actionDefinition(ActionCatalogDelete, "Delete a custom mask", controlaction.EffectWrite, deleteInputSchema, deleteSchema), invoke: invokeDelete},
		maskAction{definition: actionDefinition(ActionSelectionGet, "Get the bound session mask", controlaction.EffectRead, selectionGetInputSchema, selectionSchema), invoke: invokeSelectionGet},
		maskAction{definition: actionDefinition(ActionSelectionSet, "Set the bound session mask", controlaction.EffectWrite, selectionSetInputSchema, selectionSchema), invoke: invokeSelectionSet},
	}
}

func actionDefinition(id, description string, effect controlaction.Effect, input, result json.RawMessage) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		InputSchema: input, ResultSchema: result, MaxInputBytes: maxActionInput, MaxOutputBytes: maxActionOutput,
	}
}

type listInput struct {
	AfterID string `json:"after_id"`
	Limit   int    `json:"limit"`
}

type getInput struct {
	ID string `json:"id"`
}

type selectionInput struct {
	SessionID string `json:"session_id"`
}

type selectionSetInput struct {
	SessionID        string `json:"session_id"`
	MaskID           string `json:"mask_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

func decodeAction[T any](raw json.RawMessage, dst *T) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return maskcontract.NewError(maskcontract.CodeInvalidMask, errors.New("trailing JSON"))
		}
		return maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	return nil
}

func marshalAction(value any) (json.RawMessage, error) {
	result, err := json.Marshal(value)
	if err != nil || len(result) > maxActionOutput {
		if err == nil {
			err = errors.New("mask action output exceeds limit")
		}
		return nil, maskcontract.NewError(maskcontract.CodeUnavailable, err)
	}
	return result, nil
}

func invokeList(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in listInput
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	page, err := host.ListMasks(ctx, maskcontract.ListRequest{AfterID: in.AfterID, Limit: in.Limit})
	if err != nil {
		return nil, err
	}
	return marshalAction(page)
}

func invokeGet(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in getInput
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	definition, err := host.GetMask(ctx, maskcontract.GetRequest{ID: in.ID})
	if err != nil {
		return nil, err
	}
	return marshalAction(definition)
}

func invokeCreate(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in maskcontract.CreateRequest
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	definition, err := host.CreateMask(ctx, in)
	if err != nil {
		return nil, err
	}
	return marshalAction(definition)
}

func invokeUpdate(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in maskcontract.UpdateRequest
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	definition, err := host.UpdateMask(ctx, in)
	if err != nil {
		return nil, err
	}
	return marshalAction(definition)
}

func invokeDelete(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in maskcontract.DeleteRequest
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	deleted, err := host.DeleteMask(ctx, in)
	if err != nil {
		return nil, err
	}
	return marshalAction(deleted)
}

func invokeSelectionGet(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in selectionInput
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	view, err := host.GetMaskSelection(ctx, maskcontract.SelectionRequest{SessionID: domain.SessionID(in.SessionID)})
	if err != nil {
		return nil, err
	}
	return marshalAction(selectionResult(view))
}

func invokeSelectionSet(ctx context.Context, host maskcontract.ActionHost, raw json.RawMessage) (json.RawMessage, error) {
	var in selectionSetInput
	if err := decodeAction(raw, &in); err != nil {
		return nil, err
	}
	view, err := host.SetMaskSelection(ctx, maskcontract.SetSelectionRequest{SessionID: domain.SessionID(in.SessionID), MaskID: in.MaskID, ExpectedRevision: in.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	return marshalAction(selectionResult(view))
}

// selectionResult is the stable wire DTO. The internal SelectionView keeps
// the selection grouped for service callers, while the action contract is a
// flat object so clients never need to know that implementation detail.
type selectionWire struct {
	SessionID      string `json:"session_id"`
	MaskID         string `json:"mask_id"`
	Revision       int64  `json:"revision"`
	Available      bool   `json:"available"`
	InactiveReason string `json:"inactive_reason"`
}

func selectionResult(view maskcontract.SelectionView) selectionWire {
	return selectionWire{
		SessionID:      string(view.Selection.SessionID),
		MaskID:         view.Selection.MaskID,
		Revision:       view.Selection.Revision,
		Available:      view.Available,
		InactiveReason: view.InactiveReason,
	}
}

var (
	listInputSchema         = json.RawMessage(`{"type":"object","properties":{"after_id":{"type":"string","maxLength":128},"limit":{"type":"integer","minimum":0,"maximum":100}},"additionalProperties":false}`)
	getInputSchema          = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128}},"required":["id"],"additionalProperties":false}`)
	createInputSchema       = json.RawMessage(`{"type":"object","properties":{"operation_id":{"type":"string","pattern":"^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"},"name":{"type":"string","minLength":1,"maxLength":128},"description":{"type":"string","maxLength":1024},"body":{"type":"string","minLength":1,"maxLength":16384}},"required":["operation_id","name","description","body"],"additionalProperties":false}`)
	updateInputSchema       = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"expected_revision":{"type":"integer","minimum":1,"maximum":9007199254740991},"name":{"type":"string","minLength":1,"maxLength":128},"description":{"type":"string","maxLength":1024},"body":{"type":"string","minLength":1,"maxLength":16384}},"required":["id","expected_revision","name","description","body"],"additionalProperties":false}`)
	deleteInputSchema       = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"expected_revision":{"type":"integer","minimum":1,"maximum":9007199254740991}},"required":["id","expected_revision"],"additionalProperties":false}`)
	selectionGetInputSchema = json.RawMessage(`{"type":"object","properties":{"session_id":{"type":"string","minLength":1,"maxLength":128}},"required":["session_id"],"additionalProperties":false}`)
	selectionSetInputSchema = json.RawMessage(`{"type":"object","properties":{"session_id":{"type":"string","minLength":1,"maxLength":128},"mask_id":{"type":"string","maxLength":128},"expected_revision":{"type":"integer","minimum":0,"maximum":9007199254740991}},"required":["session_id","mask_id","expected_revision"],"additionalProperties":false}`)
	definitionSchema        = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"},"body":{"type":"string"},"revision":{"type":"integer","minimum":1,"maximum":9007199254740991},"digest":{"type":"string"},"built_in":{"type":"boolean"},"generation_id":{"type":"string"}},"required":["id","name","description","body","revision","digest","built_in","generation_id"],"additionalProperties":false}`)
	pageSchema              = json.RawMessage(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"},"digest":{"type":"string"},"generation_id":{"type":"string"},"revision":{"type":"integer","minimum":1,"maximum":9007199254740991},"built_in":{"type":"boolean"}},"required":["id","name","description","digest","generation_id","revision","built_in"],"additionalProperties":false}},"next_after_id":{"type":"string"}},"required":["items","next_after_id"],"additionalProperties":false}`)
	deleteSchema            = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`)
	selectionSchema         = json.RawMessage(`{"type":"object","properties":{"session_id":{"type":"string"},"mask_id":{"type":"string"},"revision":{"type":"integer","minimum":0,"maximum":9007199254740991},"available":{"type":"boolean"},"inactive_reason":{"type":"string","enum":["","not_compiled"]}},"required":["session_id","mask_id","revision","available","inactive_reason"],"additionalProperties":false}`)
)
