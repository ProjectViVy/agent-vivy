package memory

import (
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/controlaction"
	"agent-vivy/sdk/port/observer"
	"agent-vivy/sdk/port/tool"
)

// These typed composition helpers copy only this module's providers. A caller
// may reuse its sealed Assembly; each App keeps its own immutable Home binding.
func BindContextSource(p contextsource.Provider, s *Service) (contextsource.Provider, error) {
	if own, ok := p.(*Provider); ok {
		if !s.live() {
			return nil, errUnavailable
		}
		copy := *own
		copy.serviceBinding = serviceBinding{service: s}
		return &copy, nil
	}
	return p, nil
}
func BindRunProvider(p observer.RunProvider, s *Service) (observer.RunProvider, error) {
	if own, ok := p.(*Provider); ok {
		if !s.live() {
			return nil, errUnavailable
		}
		copy := *own
		copy.serviceBinding = serviceBinding{service: s}
		return &copy, nil
	}
	return p, nil
}
func BindToolProvider(p tool.ToolProvider, s *Service) (tool.ToolProvider, error) {
	if own, ok := p.(*memoryTool); ok {
		if !s.live() {
			return nil, errUnavailable
		}
		copy := *own
		copy.serviceBinding = serviceBinding{service: s}
		return &copy, nil
	}
	return p, nil
}
func BindActionProvider(p controlaction.Provider, s *Service) (controlaction.Provider, error) {
	if own, ok := p.(*memoryAction); ok {
		if !s.live() {
			return nil, errUnavailable
		}
		copy := *own
		copy.serviceBinding = serviceBinding{service: s}
		return &copy, nil
	}
	return p, nil
}
