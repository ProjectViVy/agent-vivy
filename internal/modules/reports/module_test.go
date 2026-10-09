package reports

import (
	"context"
	"errors"
	"testing"

	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/reportcontract"
)

type testScopes struct{}

func (testScopes) Home() nb.ScopeID { return nb.HomeScopeID }
func (testScopes) ForSession(context.Context, string) (nb.ScopeID, error) {
	return nb.WorkspaceScope("sess"), nil
}

func TestModuleDescriptorSealed(t *testing.T) {
	desc := NewModule().Descriptor()
	if desc.Module.ID != ID || len(desc.Provides) != 1 ||
		desc.Provides[0].Port != Port || desc.Provides[0].ID != ProviderID {
		t.Fatalf("descriptor drifted: %+v", desc)
	}
	if err := desc.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRequiresScopesAndGeneration(t *testing.T) {
	if _, err := Open(context.Background(), reportcontract.FactoryInput{}); err == nil {
		t.Fatal("empty factory input must fail")
	}
	if _, err := Open(context.Background(), reportcontract.FactoryInput{Scopes: testScopes{}}); err == nil {
		t.Fatal("missing generation id must fail")
	}
}

// TestServiceFailsClosedBeforeAttach: the module's Service must never
// fabricate an admission when the runtime bridge is unbound.
func TestServiceFailsClosedBeforeAttach(t *testing.T) {
	bundle, err := Open(context.Background(), reportcontract.FactoryInput{
		Scopes: testScopes{}, GenerationID: "gen-test"})
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	_, err = bundle.Service().StartReport(context.Background(),
		reportcontract.AdmissionContext{Scope: nb.HomeScopeID},
		reportcontract.ReportRequest{Period: reportcontract.PeriodDaily,
			Window: reportcontract.WindowCompleted, OperationKey: "k"})
	var rcErr *reportcontract.Error
	if !errors.As(err, &rcErr) || rcErr.Code != reportcontract.CodeCapabilityUnavailable {
		t.Fatalf("unbound admission must fail capability_unavailable, got %v", err)
	}
}

type fakeAdmission struct{ called bool }

func (f *fakeAdmission) StartReport(context.Context, reportcontract.AdmissionContext, reportcontract.ReportRequest) (reportcontract.ReportAdmission, error) {
	f.called = true
	return reportcontract.ReportAdmission{RunID: "wfr-1", Created: true}, nil
}

func (f *fakeAdmission) GetReport(context.Context, reportcontract.AdmissionContext, string) (reportcontract.ReportResult, error) {
	return reportcontract.ReportResult{Status: "active"}, nil
}

func (f *fakeAdmission) CancelReport(context.Context, reportcontract.AdmissionContext, string) error {
	return nil
}

func (f *fakeAdmission) ReadReportSettings(context.Context, reportcontract.AdmissionContext, reportcontract.Period) (reportcontract.ReportSettings, error) {
	return reportcontract.ReportSettings{Revision: 1}, nil
}

func TestAttachAdmissionDelegates(t *testing.T) {
	bundle, err := Open(context.Background(), reportcontract.FactoryInput{
		Scopes: testScopes{}, GenerationID: "gen-test"})
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	bridge := &fakeAdmission{}
	bundle.AttachAdmission(bridge)
	result, err := bundle.Service().StartReport(context.Background(),
		reportcontract.AdmissionContext{Scope: nb.HomeScopeID},
		reportcontract.ReportRequest{Period: reportcontract.PeriodDaily,
			Window: reportcontract.WindowCompleted, OperationKey: "k"})
	if err != nil || !bridge.called || !result.Created || result.RunID != "wfr-1" {
		t.Fatalf("attached bridge must delegate: %v %+v", err, result)
	}
}
