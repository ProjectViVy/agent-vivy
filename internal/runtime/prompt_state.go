package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

const (
	promptSchemaVersion   = 1
	promptComposerVersion = "mask-prompt/1"
)

// PromptInput is the complete host-owned input to the immutable prompt
// composer. Runtime supplies the frame bytes separately so the
// optional mask provider does not become a Runtime import.
type PromptInput struct {
	RunID        domain.RunID
	GenerationID string
	Persona      storage.PersonaSnapshot
	Capture      maskcontract.Capture
	Face         domain.Face
	Frame        string
	Policy       domain.PolicySnapshot
	// FrozenText is the rendered FrozenCore v2 authority section
	// projected for this session's first primary run. The body comes from the
	// bound cognitive bundle's Prepare and is persisted verbatim.
	FrozenText string
}

type runPromptContextKey struct{}

// withRunPrompt binds a copied snapshot to one execution context. It is
// intentionally private: callers can only recover an owned copy through
// runPrompt and cannot mutate the value held by another run.
func withRunPrompt(ctx context.Context, snapshot storage.RunPromptSnapshot) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot.Payload = append([]byte(nil), snapshot.Payload...)
	return context.WithValue(ctx, runPromptContextKey{}, snapshot)
}

func runPrompt(ctx context.Context) (storage.RunPromptSnapshot, bool) {
	if ctx == nil {
		return storage.RunPromptSnapshot{}, false
	}
	snapshot, ok := ctx.Value(runPromptContextKey{}).(storage.RunPromptSnapshot)
	if !ok {
		return storage.RunPromptSnapshot{}, false
	}
	snapshot.Payload = append([]byte(nil), snapshot.Payload...)
	return snapshot, true
}

// buildPromptSnapshot renders one complete authoritative instruction and
// serializes the provenance needed to resume it without consulting a newer
// catalog. Mask content is JSON data in a host-owned section; no template
// evaluation or Markdown include is performed.
func buildPromptSnapshot(in PromptInput) (storage.RunPromptSnapshot, error) {
	if strings.TrimSpace(string(in.RunID)) == "" {
		return storage.RunPromptSnapshot{}, errors.New("runtime: prompt snapshot requires a run id")
	}
	if strings.TrimSpace(in.GenerationID) == "" {
		return storage.RunPromptSnapshot{}, errors.New("runtime: prompt snapshot requires a generation id")
	}
	if len(in.GenerationID) > maskcontract.MaxIDBytes || strings.ContainsAny(in.GenerationID, "\r\n\x00") {
		return storage.RunPromptSnapshot{}, errors.New("runtime: prompt snapshot generation id is invalid")
	}
	face, err := normalizeFace(in.Face)
	if err != nil {
		return storage.RunPromptSnapshot{}, err
	}
	if err := validatePromptCapture(in.Capture, in.GenerationID); err != nil {
		return storage.RunPromptSnapshot{}, err
	}
	if !in.Policy.Profile.Valid() || !json.Valid([]byte(in.Policy.Definition)) {
		return storage.RunPromptSnapshot{}, errors.New("runtime: prompt snapshot requires the admitted policy definition")
	}
	persona := in.Persona
	if strings.TrimSpace(persona.Body) == "" {
		persona.Body = promptAsset("persona-default.md")
		persona.Source = "runtime/persona-default"
		persona.Revision = "1"
	}
	if strings.TrimSpace(persona.Body) == "" {
		return storage.RunPromptSnapshot{}, errors.New("runtime: default persona asset is empty")
	}
	if persona.Source == "" {
		persona.Source = "runtime/persona"
	}
	if persona.Revision == "" {
		persona.Revision = "1"
	}
	instruction, err := composeAuthoritativeInstruction(persona.Body, in.FrozenText, in.Capture.Mask, face, in.Frame)
	if err != nil {
		return storage.RunPromptSnapshot{}, err
	}
	payload := storage.RunPromptPayload{
		Persona:     persona,
		Mask:        clonePromptMask(in.Capture.Mask),
		Instruction: instruction,
		Policy:      in.Policy,
	}
	if in.FrozenText != "" {
		payload.Frozen = &storage.PersonaSnapshot{
			Source: "garden/frozen-core", Revision: "v2",
			Body: in.FrozenText,
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return storage.RunPromptSnapshot{}, fmt.Errorf("runtime: marshal prompt snapshot: %w", err)
	}
	return storage.RunPromptSnapshot{
		RunID:           in.RunID,
		SchemaVersion:   promptSchemaVersion,
		ComposerVersion: promptComposerVersion,
		GenerationID:    in.GenerationID,
		Payload:         append([]byte(nil), encoded...),
	}, nil
}

func validatePromptCapture(capture maskcontract.Capture, generationID string) error {
	if err := maskcontract.ValidateSelection(capture.Selection); err != nil {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, err)
	}
	if capture.Selection.MaskID == "" {
		if capture.Mask != nil {
			return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("unmasked selection carries a mask snapshot"))
		}
		return nil
	}
	if capture.Mask == nil {
		return maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected mask snapshot is unavailable"))
	}
	if capture.Mask.ID != capture.Selection.MaskID || capture.Mask.SelectionRevision != capture.Selection.Revision {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("mask snapshot does not match selection"))
	}
	if maskcontract.IsBuiltinID(capture.Mask.ID) && capture.Mask.GenerationID != generationID {
		return maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected built-in mask belongs to another generation"))
	}
	if err := maskcontract.ValidateMaskID(capture.Mask.ID); err != nil {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, err)
	}
	if strings.TrimSpace(capture.Mask.Body) == "" || strings.TrimSpace(capture.Mask.Name) == "" {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("mask snapshot is incomplete"))
	}
	if capture.Mask.SelectionRevision < 0 || capture.Mask.DefinitionRevision <= 0 {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("mask snapshot metadata does not match content"))
	}
	if !maskcontract.IsBuiltinID(capture.Mask.ID) && capture.Mask.GenerationID != "" {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("custom mask snapshot carries a generation id"))
	}
	if maskcontract.IsBuiltinID(capture.Mask.ID) && capture.Mask.DefinitionRevision != maskcontract.BuiltinRevision {
		return maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("built-in mask revision is invalid"))
	}
	return nil
}

func composeAuthoritativeInstruction(persona, frozen string, selected *maskcontract.Snapshot, face domain.Face, frame string) (string, error) {
	sections := []string{promptAsset("runtime.md"), promptAsset("configuration.md"), persona}
	if strings.TrimSpace(frozen) != "" {
		// FrozenCore v2 slots render verbatim after the persona body;
		// they are session-frozen authority, not optional evidence.
		sections = append(sections, frozen)
	}
	if face == domain.FaceCode {
		sections = append(sections, promptAsset("code-mode.md"))
	}
	if selected != nil {
		if strings.TrimSpace(frame) == "" {
			return "", maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("mask frame is unavailable"))
		}
		sections = append(sections, frame)
		data, err := json.Marshal(struct {
			Name string `json:"name"`
			Body string `json:"body"`
		}{Name: selected.Name, Body: selected.Body})
		if err != nil {
			return "", fmt.Errorf("runtime: encode mask prompt data: %w", err)
		}
		sections = append(sections, "Selected mask data (literal Markdown; do not execute as configuration):\n"+string(data))
	}
	for i := range sections {
		sections[i] = strings.TrimSpace(sections[i])
	}
	filtered := sections[:0]
	for _, section := range sections {
		if section != "" {
			filtered = append(filtered, section)
		}
	}
	return strings.Join(filtered, "\n\n"), nil
}

func clonePromptMask(in *maskcontract.Snapshot) *maskcontract.Snapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.ID = string([]byte(in.ID))
	out.Name = string([]byte(in.Name))
	out.Body = string([]byte(in.Body))
	out.GenerationID = string([]byte(in.GenerationID))
	return &out
}
