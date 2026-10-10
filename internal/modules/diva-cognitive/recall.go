package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/modules/diva-cognitive/catalog"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/contextsource"
	"github.com/ProjectViVy/laputa/garden/agentapi"
	"github.com/ProjectViVy/laputa/garden/memory"
)

const RecallModuleID = catalog.RecallModuleID
const RecallProviderID = catalog.RecallProviderID
const recallChars = 1200

// The Source is a distinct optional contribution, bound to the one owner.
// Omitting it from a Recipe physically removes ordinary automatic recall.
func NewRecallModule() module.Module { return recallModule{} }

type recallModule struct{ ownerModule }

func (recallModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: RecallModuleID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: "std/context-source@v1", ID: RecallProviderID}},
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/context-host@v1"}, Provider: "vivy/context-host"},
			{PortRef: module.PortRef{Port: Port}, Provider: ID},
		},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

type RecallSource struct {
	mu        sync.RWMutex
	owner     *bundle
	authorize func(context.Context, contextsource.Request) error
}

func NewRecallSource() *RecallSource { return &RecallSource{} }
func (*RecallSource) ID() string     { return RecallProviderID }

// BindCognitiveContext is an internal T1 App composition seam, not a public
// identity claim. Each generated Source retains only its own App's owner.
func (s *RecallSource) BindCognitiveContext(value cognitivecontract.Bundle, authorize func(context.Context, contextsource.Request) error) error {
	owner, ok := value.(*bundle)
	if !ok || owner == nil || owner.owner == nil || authorize == nil {
		return cognitivecontract.ErrUnarmed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != nil {
		return errors.New("diva-memory: source already bound")
	}
	s.owner, s.authorize = owner, authorize
	return nil
}

func (s *RecallSource) Query(ctx context.Context, request contextsource.Request) (contextsource.Page, error) {
	s.mu.RLock()
	owner, authorize := s.owner, s.authorize
	s.mu.RUnlock()
	if owner == nil || authorize == nil {
		return contextsource.Page{}, cognitivecontract.ErrUnarmed
	}
	if err := authorize(ctx, request); err != nil {
		return contextsource.Page{}, err
	}
	if strings.TrimSpace(request.Query) == "" {
		return contextsource.NewPage(nil, ""), nil
	}
	bound, err := owner.owner.BindAgentSession(request.SessionID, request.WorkspaceID)
	if err != nil {
		return contextsource.Page{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 4 {
		limit = 4
	}
	// Ordinary note kind is the native discriminator across collections.
	// Raw source_artifact material cannot substitute for a deleted note.
	cards, err := bound.SearchCards(ctx, agentapi.CardSearch{Query: request.Query, Limit: limit, Cursor: request.Cursor})
	if err != nil {
		return contextsource.Page{}, err
	}
	candidates := make([]contextsource.Candidate, 0, len(cards.Cards))
	remaining := recallChars
	for _, card := range cards.Cards {
		if card.Kind != "note" || card.Status != "active" || card.Revision <= 0 || remaining == 0 {
			continue
		}
		fragments, err := bound.ReadEvidence(ctx, agentapi.EvidenceRead{Items: []agentapi.EvidenceRef{{CardID: card.ID, ExpectedRevision: uint64(card.Revision)}}, PerItemBudget: remaining, TotalBudget: remaining})
		if err != nil {
			return contextsource.Page{}, err
		}
		used := 0
		for _, fragment := range fragments {
			if fragment.CardID != card.ID || fragment.Revision != uint64(card.Revision) || fragment.Scope != card.Scope || fragment.Status != "active" {
				return contextsource.Page{}, errors.New("diva-memory: native evidence does not match the admitted card")
			}
			used += utf8.RuneCountInString(fragment.Excerpt)
		}
		if used > remaining {
			return contextsource.Page{}, errors.New("diva-memory: native evidence exceeds the read budget")
		}
		if len(fragments) == 0 {
			continue
		}
		content, err := json.Marshal(struct {
			RecordID string                    `json:"record_id"`
			Revision int                       `json:"revision"`
			Evidence []memory.EvidenceFragment `json:"evidence"`
		}{card.ID, card.Revision, fragments})
		if err != nil {
			return contextsource.Page{}, err
		}
		remaining -= used
		candidates = append(candidates, contextsource.NewCandidate(contextsource.Candidate{SourceID: RecallProviderID, ContentID: card.ID, MediaType: "text/plain", Content: string(content), Version: strconv.Itoa(card.Revision), Confidence: min(1, max(0, card.CandidateScore)), UpdatedAt: card.ValidFrom.UnixMilli(), Treatment: contextsource.TreatmentCompetitive}))
	}
	next := ""
	if cards.NextCursor != nil {
		next = *cards.NextCursor
	}
	return contextsource.NewPage(candidates, next), nil
}
