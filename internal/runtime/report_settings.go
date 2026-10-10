package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
)

// WriteReportSettings commits one typed settings replacement: validated
// fields + revision CAS + idempotency receipt in a single Storage
// transaction, through the shared notebook mutation receipt mechanism.
// Scope and series identity are immutable — they come from the trusted
// admission context, never from caller JSON.
func (s *Service) WriteReportSettings(ctx context.Context, ac rc.AdmissionContext, keyed nb.OperationKeyed[rc.ReportSettingsWrite]) (rc.ReportSettingsWriteResult, error) {
	if s.deps.Report == nil {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeCapabilityUnavailable,
			Message: "report settings capability is not configured"}
	}
	in := keyed.Request
	if ac.Scope == "" || ac.Actor.Kind == "" || ac.Origin == "" {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
			Message: "settings write requires bound scope, actor and origin"}
	}
	if !in.Period.Valid() {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest, Message: "invalid period"}
	}
	if keyed.OperationKey == "" {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest, Message: "operation_key is required"}
	}
	if in.ExpectedRevision <= 0 {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
			Message: "expected_revision is required"}
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
			Message: "timezone is not an IANA zone: " + err.Error()}
	}
	expr := strings.TrimSpace(in.ScheduleExpr)
	if in.Enabled && expr == "" {
		return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
			Message: "schedule_expr is required when enabled"}
	}
	if expr != "" {
		if err := ValidateCronExpr(expr); err != nil {
			return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
				Message: "invalid schedule_expr: " + err.Error()}
		}
	}
	if err := s.validateReportSection(ctx, string(ac.Scope), in.SectionID); err != nil {
		return rc.ReportSettingsWriteResult{}, err
	}
	if err := s.validateReportModelSelection(in.Provider, in.ModelID); err != nil {
		return rc.ReportSettingsWriteResult{}, err
	}
	if _, err := s.reportSettings(ctx, string(ac.Scope), in.Period); err != nil {
		return rc.ReportSettingsWriteResult{}, err
	}
	settings := rc.ReportSettings{
		Scope: string(ac.Scope), Period: in.Period, Timezone: in.Timezone,
		SectionID: in.SectionID, Provider: in.Provider, ModelID: in.ModelID,
		Enabled: in.Enabled, ScheduleExpr: expr,
	}
	var nextRunAtMs int64
	if in.Enabled {
		nextRunAtMs = NextCronAfter(reportCronSchedule(settings), time.Now().UnixMilli())
		if nextRunAtMs <= 0 {
			return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeInvalidRequest,
				Message: "schedule_expr has no future occurrence"}
		}
	}
	mc := nb.MutationContext{ScopeID: ac.Scope, Actor: ac.Actor, OperationKey: keyed.OperationKey}
	committed, receipt, err := s.deps.Report.CommitReportSettings(ctx, mc, in.ExpectedRevision, settings, nextRunAtMs)
	if err != nil {
		if errors.Is(err, storage.ErrRevisionConflict) {
			return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeIdempotencyConflict,
				Message: "settings revision is stale"}
		}
		if errors.Is(err, storage.ErrNotFound) {
			return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeNotFound,
				Message: "report settings row not found"}
		}
		if errors.Is(err, nb.ErrIdempotencyConflict) {
			return rc.ReportSettingsWriteResult{}, &rc.Error{Code: rc.CodeIdempotencyConflict,
				Message: "operation key was committed with a different settings write"}
		}
		return rc.ReportSettingsWriteResult{}, err
	}
	s.KickCronScheduler()
	return rc.ReportSettingsWriteResult{Settings: committed, Replayed: receipt.Replayed}, nil
}

// reportCronSchedule maps settings onto the durable cron schedule shape
// the host evaluates (report rows are always cron-kind).
func reportCronSchedule(settings rc.ReportSettings) domain.CronSchedule {
	return domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: settings.ScheduleExpr, TZ: settings.Timezone}
}

// validateReportSection requires the destination section to exist in the
// scope and not be deleted; the notebook store is the only authority.
func (s *Service) validateReportSection(ctx context.Context, scope, sectionID string) error {
	if sectionID == "" {
		return &rc.Error{Code: rc.CodeInvalidRequest, Message: "section_id is required"}
	}
	nbs, ok := s.deps.Sessions.(interface {
		Notebook() storage.NotebookStore
	})
	if !ok || nbs.Notebook() == nil {
		return &rc.Error{Code: rc.CodeStorageUnavailable, Message: "notebook store is unavailable"}
	}
	page, err := nbs.Notebook().ListSections(ctx, nb.ScopeID(scope), nb.ListSectionsRequest{Limit: 500})
	if err != nil {
		return &rc.Error{Code: rc.CodeStorageUnavailable, Message: err.Error()}
	}
	for _, sec := range page.Sections {
		if sec.ID == sectionID {
			if sec.DeletedAt != 0 {
				return &rc.Error{Code: rc.CodeInvalidRequest, Message: "section is deleted"}
			}
			return nil
		}
	}
	return &rc.Error{Code: rc.CodeInvalidRequest, Message: "section is not authorized in this scope"}
}

// validateReportModelSelection requires a provider the embedded catalog
// declares; a model id is only valid alongside its provider and when some
// endpoint of that vendor declares it.
func (s *Service) validateReportModelSelection(providerName, modelID string) error {
	if providerName == "" {
		if modelID != "" {
			return &rc.Error{Code: rc.CodeInvalidRequest, Message: "model_id requires provider"}
		}
		return nil
	}
	if s.catalog == nil {
		return &rc.Error{Code: rc.CodeInvalidRequest, Message: "provider catalog is unavailable"}
	}
	vendor, ok := s.catalog.Vendor(providerName)
	if !ok {
		return &rc.Error{Code: rc.CodeInvalidRequest, Message: "provider is not in the catalog"}
	}
	if modelID == "" {
		return nil
	}
	for _, ep := range vendor.Endpoints {
		if _, declared := ep.Model(modelID); declared {
			return nil
		}
	}
	return &rc.Error{Code: rc.CodeInvalidRequest, Message: "model_id is not declared by the provider"}
}
