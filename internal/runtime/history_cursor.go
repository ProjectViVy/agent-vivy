package runtime

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Cursor values are bounded request positions, never proof of issuance or scope.
// Every use compares server-normalized inputs and recaptures authorized bounds.
type historyCursorState struct {
	Version     int                     `json:"version"`
	Destination string                  `json:"destination"`
	Scope       []domain.SessionID      `json:"scope"`
	Filter      historyCursorFilter     `json:"filter"`
	Cut         storage.HistoryCut      `json:"cut"`
	Phase       string                  `json:"phase"`
	After       storage.HistoryPosition `json:"after"`
}

type historyCursorFilter struct {
	Kind       string                   `json:"kind"`
	Query      string                   `json:"query,omitempty"`
	From       int64                    `json:"from,omitempty"`
	To         int64                    `json:"to,omitempty"`
	Kinds      []string                 `json:"kinds,omitempty"`
	ArtifactID string                   `json:"artifact_id,omitempty"`
	Selection  *domain.HistorySelection `json:"selection,omitempty"`
}

func (c historyCursorState) done() bool { return c.Phase == historyPhaseDone }

func normalizeHistoryCursorFilter(request any) historyCursorFilter {
	switch r := request.(type) {
	case domain.HistorySearchRequest:
		kinds := append([]string(nil), r.Kinds...)
		slices.Sort(kinds)
		kinds = slices.Compact(kinds)
		return historyCursorFilter{Kind: "search", Query: strings.ToLower(r.Query), From: r.From, To: r.To, Kinds: kinds, ArtifactID: r.ArtifactID}
	case domain.HistoryReadRequest:
		filter := historyCursorFilter{Kind: "read"}
		if r.Selection != nil {
			selection := *r.Selection
			selection.Refs = append([]domain.SourceRef(nil), selection.Refs...)
			slices.SortFunc(selection.Refs, func(a, b domain.SourceRef) int {
				return cmp.Or(cmp.Compare(a.SessionID, b.SessionID), cmp.Compare(a.RunID, b.RunID), cmp.Compare(a.MessageID, b.MessageID), cmp.Compare(a.EventSeq, b.EventSeq), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.CreatedAt, b.CreatedAt))
			})
			selection.Refs = slices.Compact(selection.Refs)
			if selection.RunRange != nil {
				value := *selection.RunRange
				selection.RunRange = &value
			}
			filter.Selection = &selection
		}
		return filter
	}
	return historyCursorFilter{}
}
func sameHistoryCursorRequest(state historyCursorState, sessions []domain.SessionID, filter historyCursorFilter) bool {
	return slices.Equal(state.Scope, sessions) && reflect.DeepEqual(state.Filter, filter)
}

func (s *HistoryService) encodeCursor(cursor historyCursorState) (string, error) {
	cursor.Version = historyCursorVersion
	raw, err := json.Marshal(cursor)
	if err != nil || len(raw) > historyCursorDecodedMax {
		return "", errHistoryCursorStale
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > historyCursorTransportMax {
		return "", errHistoryCursorStale
	}
	return encoded, nil
}
func (s *HistoryService) decodeCursor(encoded string) (historyCursorState, error) {
	bad := func() (historyCursorState, error) { return historyCursorState{}, errHistoryCursorStale }
	if len(encoded) == 0 || len(encoded) > historyCursorTransportMax {
		return bad()
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw) > historyCursorDecodedMax {
		return bad()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var state historyCursorState
	if err := decoder.Decode(&state); err != nil {
		return bad()
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return bad()
	}
	if state.Version != historyCursorVersion || state.Destination == "" || !utf8.ValidString(state.Destination) || len(state.Destination) > storage.HistoryMetadataIdentityBytesMax {
		return bad()
	}
	accepted, err := domain.NewAcceptedHistoryScope(domain.SessionID(state.Destination), state.Scope)
	if err != nil || !slices.Equal(accepted.SourceSessionIDs, state.Scope) {
		return bad()
	}
	if err := state.Cut.Validate(); err != nil {
		return bad()
	}
	if err := state.After.Validate(); err != nil {
		return bad()
	}
	if state.After.IsZero() && state.After != (storage.HistoryPosition{}) {
		return bad()
	}
	switch state.Phase {
	case historyPhaseMessages:
		if !state.Cut.ContainsMessagePosition(state.After) {
			return bad()
		}
	case historyPhaseEvents:
		if !state.Cut.ContainsRunEventPosition(state.After) {
			return bad()
		}
	case historyPhaseDone:
		if !state.After.IsZero() {
			return bad()
		}
	default:
		return bad()
	}
	return state, nil
}

func (s *HistoryService) validateHistoryCursorCut(ctx context.Context, state historyCursorState, sessions []domain.SessionID) error {
	if len(state.Cut.Sessions) != len(sessions) {
		return errors.New("history cursor cut differs from selected scope")
	}
	current, err := s.query.CaptureHistoryCut(ctx, sessions)
	if err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if len(current.Sessions) != len(sessions) {
		return errors.New("history current cut differs from selected scope")
	}
	for i, cut := range state.Cut.Sessions {
		if cut.SessionID != sessions[i] || current.Sessions[i].SessionID != cut.SessionID || cut.Position > current.Sessions[i].Position {
			return errors.New("history cursor session position is outside authorized cut")
		}
	}
	runs := make(map[domain.RunID]storage.HistoryRunCut, len(current.Runs))
	for _, run := range current.Runs {
		runs[run.RunID] = run
	}
	for _, run := range state.Cut.Runs {
		authorized, ok := runs[run.RunID]
		if !ok || run.SessionID != authorized.SessionID || run.Seq > authorized.Seq {
			return errors.New("history cursor run position is outside authorized cut")
		}
	}
	return nil
}
