package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	action "agent-vivy/sdk/port/controlaction"
)

// The wire projection exposes the entire owner text, never its private receipt
// envelope or a caller-selectable identity. Sources are expanded by Service.
type recapHeadView struct {
	Revision          uint64    `json:"revision"`
	Text              string    `json:"text"`
	CoveredThrough    uint64    `json:"covered_through"`
	InvalidationEpoch uint64    `json:"invalidation_epoch"`
	Cause             string    `json:"cause"`
	RestoredFrom      uint64    `json:"restored_from,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func recapView(h cognitivecontract.RecapHead) *recapHeadView {
	return &recapHeadView{h.Revision, h.Text, h.CoveredThrough, h.InvalidationEpoch, h.Cause, h.RestoredFrom, h.UpdatedAt}
}

type recapOutcome struct {
	Status       string           `json:"status"`
	Head         *recapHeadView   `json:"head,omitempty"`
	Revisions    []*recapHeadView `json:"revisions,omitempty"`
	NextBefore   uint64           `json:"next_before,omitempty"`
	Reason       string           `json:"reason,omitempty"`
	Activation   string           `json:"activation"`
	LoadedStatus string           `json:"loaded_status"`
}
type recapInput struct {
	SessionID    string  `json:"session_id"`
	Text         *string `json:"text,omitempty"`
	BaseRevision *uint64 `json:"base_revision,omitempty"`
	OperationID  string  `json:"operation_id,omitempty"`
	Revision     *uint64 `json:"revision,omitempty"`
	Before       uint64  `json:"before,omitempty"`
	Limit        *int    `json:"limit,omitempty"`
}

func recapActionDefinitions() []action.Definition {
	defs := []action.Definition{}
	for _, name := range []string{"read", "edit", "clear", "history", "revision", "restore", "sources"} {
		effect := action.EffectRead
		props := []string{sessionProp}
		required := req()
		switch name {
		case "edit", "clear", "restore":
			effect = action.EffectWrite
			props = append(props, prop("base_revision", numProp), prop("operation_id", `{"type":"string","minLength":1}`))
			required = req("base_revision", "operation_id")
			if name == "edit" {
				props = append(props, prop("text", strProp))
				required = append(required, "text")
			}
			if name == "restore" {
				props = append(props, prop("revision", `{"type":"integer","minimum":1}`))
				required = append(required, "revision")
			}
		case "revision", "sources":
			props = append(props, prop("revision", `{"type":"integer","minimum":1}`))
			required = req("revision")
		case "history":
			props = append(props, prop("before", numProp), prop("limit", `{"type":"integer","minimum":1,"maximum":50}`))
		}
		defs = append(defs, actionDef("vivy.recap."+name, "Manage the complete subject Recap: "+name, effect, schema(required, props...)))
	}
	return defs
}

func (d dispatch) authorizedRecap(ctx context.Context, sid domain.SessionID, tool string) (cognitivecontract.RecapPort, error) {
	if d.bundle == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	management, ok := control.(cognitivecontract.RecapManagementPort)
	if !ok || d.bundle.recap == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	rid := domain.RunID("")
	if tool != "" {
		rid = tools.RunIDFromContext(ctx)
		if rid == "" || tools.SessionIDFromContext(ctx) != sid {
			return nil, errors.New("recap_scope_denied")
		}
	}
	binding, err := management.AuthorizeRecap(ctx, sid, rid, tool)
	if err != nil {
		return nil, errors.New("recap_scope_denied")
	}
	head, err := d.bundle.recap.ReadRecap(ctx)
	if err != nil {
		return nil, err
	}
	if head.Binding != binding {
		return nil, errors.New("recap_scope_denied")
	}
	return d.bundle.recap, nil
}
func recapFailure(err error) recapOutcome {
	reason := "recap_unavailable"
	if err != nil {
		if err.Error() == "recap_scope_denied" {
			reason = "recap_scope_denied"
		} else if code, _, _ := classify(err); code != "internal" {
			reason = code
		}
	}
	return recapOutcome{Status: "failed", Reason: reason, Activation: "next_run", LoadedStatus: "unavailable"}
}
func (d dispatch) invokeRecap(ctx context.Context, id string, raw json.RawMessage, tool string) (json.RawMessage, error) {
	if err := strictRecapJSON(raw); err != nil {
		return nil, err
	}
	var in recapInput
	if err := decodeInput(raw, &in); err != nil {
		return nil, action.ErrInvalidInput
	}
	name := strings.TrimPrefix(id, "vivy.recap.")
	var definition *action.Definition
	for _, def := range recapActionDefinitions() {
		if def.ID == id {
			copy := def
			definition = &copy
			break
		}
	}
	if definition == nil {
		return nil, action.ErrInvalidInput
	}
	// Validate against the same closed schema even for private dispatcher callers.
	if err := tools.ValidateArgs(domain.ToolSpec{Schema: definition.InputSchema}, raw); err != nil {
		return nil, action.ErrInvalidInput
	}
	if (name == "edit" || name == "clear" || name == "restore") && strings.TrimSpace(in.OperationID) == "" {
		return nil, action.ErrInvalidInput
	}
	port, err := d.authorizedRecap(ctx, domain.SessionID(in.SessionID), tool)
	if err != nil {
		return json.Marshal(recapFailure(err))
	}
	out := recapOutcome{Status: "listed", Activation: "next_run", LoadedStatus: "unavailable"}
	switch name {
	case "read":
		var h cognitivecontract.RecapHead
		h, err = port.ReadRecap(ctx)
		if err == nil {
			out.Head = recapView(h)
		}
	case "history":
		limit := 20
		if in.Limit != nil {
			limit = *in.Limit
		}
		var heads []cognitivecontract.RecapHead
		heads, err = port.ListRecapHistory(ctx, in.Before, limit)
		out.Revisions = []*recapHeadView{}
		if err == nil {
			for _, h := range heads {
				out.Revisions = append(out.Revisions, recapView(h))
			}
			if len(heads) == limit {
				out.NextBefore = heads[len(heads)-1].Revision
			}
		}
	case "revision", "sources":
		var h cognitivecontract.RecapHead
		h, err = port.ReadRecapRevision(ctx, *in.Revision)
		if err == nil {
			if name == "sources" {
				out.Status = "failed"
				out.Reason = "recap_sources_unavailable"
			} else {
				out.Head = recapView(h)
			}
		}
	case "edit", "clear", "restore":
		request := cognitivecontract.RecapCommit{BaseRevision: *in.BaseRevision, OperationID: in.OperationID, Cause: name}
		if name == "edit" {
			request.Text = *in.Text
		}
		if name == "restore" {
			request.RestoredFrom = *in.Revision
		}
		var h cognitivecontract.RecapHead
		h, err = port.CommitRecap(ctx, request)
		if err == nil {
			out.Status = "applied"
			out.Head = recapView(h)
			if name == "clear" {
				out.Activation = "next_safe_boundary"
			}
		}
		if code, _, _ := classify(err); err != nil && code == "recap_conflict" {
			// Fetch once under fresh authorization; human text never gets a CAS retry.
			port, authErr := d.authorizedRecap(ctx, domain.SessionID(in.SessionID), tool)
			if authErr != nil {
				return json.Marshal(recapFailure(authErr))
			}
			h, err = port.ReadRecap(ctx)
			if err == nil {
				out.Status = "conflict"
				out.Reason = "recap_conflict"
				out.Head = recapView(h)
			}
		}
	}
	if err != nil {
		out = recapFailure(err)
	}
	return json.Marshal(out)
}
