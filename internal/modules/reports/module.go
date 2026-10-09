// Package reports is the build-linked T1 Module owning the bounded report
// capability (R0). Construction is side-effect free: the sealed factory
// seam opens the single owner, which binds the trusted scope resolver and
// the runtime admission bridge — never a raw DB or untyped locator.
package reports

import (
	"context"
	"errors"
	"sync"

	"agent-vivy/internal/reportcontract"
	"agent-vivy/sdk/module"
)

const (
	// ID is this Module's identity in the build-owned Source Catalog.
	ID = "vivy/reports"
	// Port is the closed internal Port it provides.
	Port = "core/report-service@v1"
	// ProviderID is the sole provider identity under the Port.
	ProviderID = "vivy.report-service"
)

type owner struct{}
type instance struct{}

// NewModule returns the Module owner used by the generated Assembly.
func NewModule() module.Module { return owner{} }

// Descriptor declares the closed Port it provides. The catalog adds the
// core/action-host@v1 and core/notebook-service@v1 Requires edges.
func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal"},
		Provides:   []module.PortRef{{Port: Port, ID: ProviderID}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }

// bundle carries the scoped Service plus the deferred admission bridge.
type bundle struct {
	mu        sync.RWMutex
	admission reportcontract.AdmissionPort
	scopes    reportcontract.ScopeResolver
	genID     string
}

// Open is the sealed factory invoked by the generated RuntimeAssembly.
func Open(ctx context.Context, in reportcontract.FactoryInput) (reportcontract.Bundle, error) {
	if in.Scopes == nil || in.GenerationID == "" {
		return nil, errors.New("reports: factory input is incomplete")
	}
	return &bundle{scopes: in.Scopes, genID: in.GenerationID}, nil
}

func (b *bundle) AttachAdmission(port reportcontract.AdmissionPort) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.admission = port
}

func (b *bundle) Service() reportcontract.Service { return &service{bundle: b} }

func (b *bundle) Close() error { return nil }

// service is the sealed report capability surface (R0: admission only).
type service struct {
	bundle *bundle
}

func (s *service) StartReport(ctx context.Context, ac reportcontract.AdmissionContext, req reportcontract.ReportRequest) (reportcontract.ReportAdmission, error) {
	s.bundle.mu.RLock()
	admission := s.bundle.admission
	s.bundle.mu.RUnlock()
	if admission == nil {
		return reportcontract.ReportAdmission{}, &reportcontract.Error{
			Code:    reportcontract.CodeCapabilityUnavailable,
			Message: "report admission is not bound",
		}
	}
	return admission.StartReport(ctx, ac, req)
}
