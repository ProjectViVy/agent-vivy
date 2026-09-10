package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	porttool "agent-vivy/sdk/port/tool"
)

var (
	ErrDuplicateToolID = errors.New("duplicate tool id")
	ErrProtectedToolID = errors.New("protected tool id collision")
	ErrUnknownToolID   = errors.New("unknown tool id")
	ErrInvalidTool     = errors.New("invalid tool binding")
)

type StaticBinding struct {
	OwnerID  string
	Provider porttool.ToolProvider
	Host     porttool.Host
}

type Config struct {
	Static       []StaticBinding
	ProtectedIDs []string
}

type Request struct {
	ID   string
	Args json.RawMessage
}

type Host struct {
	static    map[string]StaticBinding
	visible   []porttool.Definition
	protected map[string]struct{}
}

func New(cfg Config) (*Host, error) {
	h := &Host{
		static:    make(map[string]StaticBinding, len(cfg.Static)),
		protected: make(map[string]struct{}, len(cfg.ProtectedIDs)),
	}
	for _, id := range cfg.ProtectedIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			h.protected[id] = struct{}{}
		}
	}
	for _, binding := range cfg.Static {
		if binding.Provider == nil || binding.Host == nil {
			return nil, ErrInvalidTool
		}
		definition := binding.Provider.Definition()
		definition.ID = strings.TrimSpace(definition.ID)
		if definition.ID == "" {
			return nil, ErrInvalidTool
		}
		if _, exists := h.static[definition.ID]; exists {
			if h.IsProtected(definition.ID) {
				return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, definition.ID)
			}
			return nil, fmt.Errorf("%w: %s", ErrDuplicateToolID, definition.ID)
		}
		h.static[definition.ID] = binding
		h.visible = append(h.visible, definition)
	}
	sort.Slice(h.visible, func(i, j int) bool {
		return h.visible[i].ID < h.visible[j].ID
	})
	return h, nil
}

func (h *Host) IsProtected(id string) bool {
	_, ok := h.protected[strings.TrimSpace(id)]
	return ok
}

func (h *Host) ListVisible() []porttool.Definition {
	return append([]porttool.Definition(nil), h.visible...)
}

func (h *Host) Invoke(ctx context.Context, req Request) (porttool.Result, error) {
	binding, ok := h.static[strings.TrimSpace(req.ID)]
	if !ok {
		return porttool.Result{}, fmt.Errorf("%w: %s", ErrUnknownToolID, req.ID)
	}
	return binding.Provider.Invoke(ctx, binding.Host, req.Args)
}
