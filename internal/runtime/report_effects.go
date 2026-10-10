package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"

	inofy "github.com/ProjectViVy/inofy"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Bounded collection limits admitted by the settings row (R1 fixed
// defaults; R3 owns writes). Every number is a hard ceiling: collection
// never reads or emits beyond them and truncation is always disclosed.
const (
	reportMaxSessions          = 200
	reportMaxMessagesPerSess   = 400
	reportMaxFacts             = 400
	reportFactBytes            = 2048
	reportMaxFeedback          = 64
	reportNarrateMaxTokens     = 2048
	reportNarrateTimeoutMS     = 120000
	reportMaxMarkdownBytes     = 64 * 1024
	reportPersistMaxAttempts   = 2
	reportDailyFallbackMinFact = 1
)

// nodeIn decodes the bound single-input envelope the sealed program feeds
// each node: {"input": <upstream packet>}.
type reportNodeInput struct {
	Input json.RawMessage `json:"input"`
}

func decodeNodeInput[T any](call inofy.NodeCall) (T, error) {
	var zero T
	var env reportNodeInput
	if err := json.Unmarshal(call.Input, &env); err != nil {
		return zero, fmt.Errorf("decode node input envelope: %w", err)
	}
	if len(env.Input) == 0 {
		return zero, errors.New("node input is empty")
	}
	var v T
	if err := json.Unmarshal(env.Input, &v); err != nil {
		return zero, fmt.Errorf("decode node input: %w", err)
	}
	return v, nil
}

func reportReply(v any) (inofy.NodeReply, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return inofy.NodeReply{}, err
	}
	if len(raw) > maxINOFYNodeResultBytes {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrNodeFailed, Message: "report node output exceeds the bounded result size"}
	}
	return inofy.NodeReply{Output: raw}, nil
}

func digestJSON(kind string, v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte(kind+"\n"), raw...))
	return hex.EncodeToString(sum[:])
}

// --- collect --------------------------------------------------------------

type reportCollectOutput struct {
	Pins   reportRunInput `json:"pins"`
	Bundle rc.FactBundle  `json:"bundle"`
}

func (e *reportNodeExecutor) effectCollect(ctx context.Context, call inofy.NodeCall) (inofy.NodeReply, error) {
	pins, err := decodeNodeInput[reportRunInput](call)
	if err != nil {
		return reportErrReply(call, inofy.ErrInvalidDefinition, err.Error())
	}
	store := e.svc.deps.Report
	if store == nil {
		return reportErrReply(call, inofy.ErrAuthorityDenied, "report source capability is not configured")
	}
	bundle := rc.FactBundle{
		Window:         pins.rcWindow(),
		SeriesID:       pins.SeriesID,
		ConfigRevision: pins.ConfigRevision,
	}

	// Target + prior-series + reused daily docs form the feedback set.
	targetIDs := map[string]struct{}{}
	if pins.Target != nil && pins.Target.EntryID != "" {
		targetIDs[pins.Target.EntryID] = struct{}{}
	}
	var priorEntryID string
	if pins.Period != rc.PeriodDaily {
		if prior, perr := store.ListReportGenerations(ctx, pins.Scope, pins.SeriesID, "", pins.WindowID); perr == nil {
			for i := len(prior) - 1; i >= 0; i-- {
				if prior[i].WindowID != pins.WindowID && prior[i].EntryID != "" {
					priorEntryID = prior[i].EntryID
					break
				}
			}
		}
	}
	if priorEntryID != "" {
		targetIDs[priorEntryID] = struct{}{}
	}

	// Facts: daily reads session activity; weekly/monthly reuse committed
	// daily bundles inside the covered local days, falling back to
	// sessions for uncovered dates.
	dailyGens := map[string]storage.ReportGeneration{}
	if pins.Period == rc.PeriodWeekly || pins.Period == rc.PeriodMonthly {
		loc, _ := time.LoadLocation(pins.Timezone)
		if loc == nil {
			loc = time.UTC
		}
		days := coveredLocalDays(pins.rcWindow(), loc)
		covered := map[string]bool{}
		rows, gerr := store.ListReportGenerations(ctx, pins.Scope, reportSeriesID(rc.PeriodDaily), "", "\xff")
		if gerr != nil {
			return reportErrReply(call, inofy.ErrStorageFailed, gerr.Error())
		}
		for _, g := range rows {
			dailyGens[g.WindowID] = g
		}
		for _, day := range days {
			if g, ok := dailyGens[day]; ok && g.EntryID != "" {
				covered[day] = true
				targetIDs[g.EntryID] = struct{}{}
				var sub rc.FactBundle
				if json.Unmarshal(g.FactsJSON, &sub) == nil {
					for _, f := range sub.Facts {
						if len(bundle.Facts) >= reportMaxFacts {
							bundle.Truncated++
							continue
						}
						bundle.Facts = append(bundle.Facts, f)
						bundle.Included++
					}
					bundle.Excluded += sub.Excluded
				}
				bundle.Facts = append(bundle.Facts, rc.Fact{
					Date:    day,
					Text:    "Daily report for " + day,
					Sources: []rc.SourceRef{{Kind: "report", ID: g.RunID, Digest: g.FactsDigest}},
				})
			} else {
				bundle.MissingDates = append(bundle.MissingDates, day)
			}
		}
		// Session fallback for uncovered days inside the window.
		uncovered := []string{}
		for _, day := range days {
			if !covered[day] {
				uncovered = append(uncovered, day)
			}
		}
		if err := collectSessions(ctx, store, &bundle, pins, uncovered); err != nil {
			return reportErrReply(call, inofy.ErrStorageFailed, err.Error())
		}
	} else {
		if err := collectSessions(ctx, store, &bundle, pins, nil); err != nil {
			return reportErrReply(call, inofy.ErrStorageFailed, err.Error())
		}
	}

	// Feedback + human-edit context.
	ids := make([]string, 0, len(targetIDs))
	for id := range targetIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	feedback, ferr := store.ListReportFeedback(ctx, pins.Scope, ids)
	if ferr != nil {
		return reportErrReply(call, inofy.ErrStorageFailed, ferr.Error())
	}
	for i, fb := range feedback {
		if i >= reportMaxFeedback {
			bundle.Omitted += len(feedback) - i
			break
		}
		sum := sha256.Sum256([]byte(fb.Body))
		bundle.Feedback = append(bundle.Feedback, rc.FeedbackSnapshot{
			CommentID: fb.CommentID, EntryID: fb.EntryID, Version: fb.Version,
			Body: fb.Body, Digest: hex.EncodeToString(sum[:]),
		})
	}
	edits, eerr := store.ListReportHumanEdits(ctx, pins.Scope, ids, reportMaxFeedback)
	if eerr != nil {
		return reportErrReply(call, inofy.ErrStorageFailed, eerr.Error())
	}
	for _, ed := range edits {
		bundle.HumanEdits = append(bundle.HumanEdits, rc.FeedbackSnapshot{
			CommentID: ed.CommentID, EntryID: ed.EntryID, Version: ed.Version, Body: ed.Body,
		})
	}
	sort.Slice(bundle.Facts, func(i, j int) bool {
		if bundle.Facts[i].Date == bundle.Facts[j].Date {
			return bundle.Facts[i].Text < bundle.Facts[j].Text
		}
		return bundle.Facts[i].Date < bundle.Facts[j].Date
	})
	return reportReply(reportCollectOutput{Pins: pins, Bundle: bundle})
}

func collectSessions(ctx context.Context, store storage.ReportSourceStore, bundle *rc.FactBundle, pins reportRunInput, days []string) error {
	sessions, err := store.ListReportSourceSessions(ctx, pins.Scope)
	if err != nil {
		return err
	}
	if len(sessions) > reportMaxSessions {
		bundle.Excluded += len(sessions) - reportMaxSessions
		sessions = sessions[:reportMaxSessions]
	}
	loc, _ := time.LoadLocation(pins.Timezone)
	if loc == nil {
		loc = time.UTC
	}
	daySet := map[string]bool{}
	for _, d := range days {
		daySet[d] = true
	}
	for _, sess := range sessions {
		msgs, err := store.ListReportSourceMessages(ctx, string(sess.ID), pins.WindowStartMs, pins.WindowEndMs, reportMaxMessagesPerSess)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			day := time.UnixMilli(m.CreatedAtMs).In(loc).Format("2006-01-02")
			if days != nil && !daySet[day] {
				continue
			}
			if len(bundle.Facts) >= reportMaxFacts {
				bundle.Truncated++
				continue
			}
			text := m.Content
			truncated := false
			if len(text) > reportFactBytes {
				text = text[:reportFactBytes]
				truncated = true
			}
			sum := sha256.Sum256([]byte(m.Content))
			bundle.Facts = append(bundle.Facts, rc.Fact{
				Date:      day,
				SessionID: string(sess.ID),
				Text:      fmt.Sprintf("%s: %s", m.Role, text),
				Sources:   []rc.SourceRef{{Kind: "message", ID: m.MessageID, Digest: hex.EncodeToString(sum[:])}},
			})
			bundle.Included++
			if truncated {
				bundle.Truncated++
			}
		}
	}
	return nil
}

// --- narrate ---------------------------------------------------------------

type reportNarrateOutput struct {
	Pins      reportRunInput `json:"pins"`
	Bundle    rc.FactBundle  `json:"bundle"`
	Narrative rc.Narrative   `json:"narrative"`
	Model     string         `json:"model,omitempty"`
	Provider  string         `json:"provider,omitempty"`
}

const reportNarratePrompt = `You are writing a %s report for %s (%s to %s, timezone %s).
Summarize the facts below into a short report. Reply with ONLY strict JSON of this shape:
{"mode":"model","sections":[{"heading":"...","claims":[{"text":"...","source_ids":["..."]}]}]}
Every claim MUST cite its source_ids from the provided source ids. Keep it under %d tokens.
Feedback from the user that should inform the tone: %s
Facts:
%s`

func (e *reportNodeExecutor) effectNarrate(ctx context.Context, call inofy.NodeCall) (inofy.NodeReply, error) {
	in, err := decodeNodeInput[reportCollectOutput](call)
	if err != nil {
		return reportErrReply(call, inofy.ErrInvalidDefinition, err.Error())
	}
	out := reportNarrateOutput{Pins: in.Pins, Bundle: in.Bundle}
	out.Provider = in.Pins.Provider
	out.Model = in.Pins.ModelID

	if len(in.Bundle.Facts) == 0 {
		out.Narrative = rc.Narrative{Mode: rc.NarrativeEmpty, Reason: "no facts in window"}
		return reportReply(out)
	}
	cm := e.svc.engine.chatModel
	if cm == nil {
		out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "model_unavailable"}
		return reportReply(out)
	}
	var facts strings.Builder
	ids := map[string]struct{}{}
	for _, f := range in.Bundle.Facts {
		for _, src := range f.Sources {
			ids[src.ID] = struct{}{}
		}
		facts.WriteString("- [" + f.Date + "] " + f.Text + " (sources:")
		for _, s2 := range f.Sources {
			facts.WriteString(" " + s2.ID)
		}
		facts.WriteString(")\n")
	}
	var fb strings.Builder
	for _, f2 := range in.Bundle.Feedback {
		fb.WriteString("- " + f2.Body + "\n")
	}
	prompt := fmt.Sprintf(reportNarratePrompt, in.Pins.Period, in.Pins.WindowID,
		time.UnixMilli(in.Pins.WindowStartMs).UTC().Format("2006-01-02"),
		time.UnixMilli(in.Pins.WindowEndMs).UTC().Format("2006-01-02"),
		in.Pins.Timezone, reportNarrateMaxTokens, fb.String(), facts.String())

	callCtx, cancel := context.WithTimeout(ctx, reportNarrateTimeoutMS*time.Millisecond)
	defer cancel()
	// Exactly one tool-free Generate on the resolved model instance; no
	// repair loops, retries, or second graphs.
	msg, gerr := cm.Generate(callCtx, []*schema.Message{
		schema.UserMessage(prompt),
	}, model.WithMaxTokens(reportNarrateMaxTokens))
	if gerr != nil {
		if errors.Is(gerr, context.Canceled) || errors.Is(gerr, context.DeadlineExceeded) && errors.Is(ctx.Err(), context.Canceled) {
			return reportErrReply(call, inofy.ErrorCode("cancelled"), "report narrate cancelled")
		}
		if errors.Is(gerr, context.DeadlineExceeded) {
			out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "model_timeout"}
			return reportReply(out)
		}
		out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "model_error: " + gerr.Error()}
		return reportReply(out)
	}
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "empty_model_output"}
		return reportReply(out)
	}
	var parsed rc.Narrative
	raw := strings.TrimSpace(msg.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	if uerr := json.Unmarshal([]byte(raw), &parsed); uerr != nil {
		out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "malformed_output"}
		return reportReply(out)
	}
	// Every cited source id must exist in the collected bundle.
	known := map[string]struct{}{}
	for _, f := range in.Bundle.Facts {
		for _, s3 := range f.Sources {
			known[s3.ID] = struct{}{}
		}
	}
	for _, sec := range parsed.Sections {
		for _, cl := range sec.Claims {
			for _, id := range cl.SourceIDs {
				if _, ok := known[id]; !ok {
					out.Narrative = rc.Narrative{Mode: rc.NarrativeFallback, Reason: "unknown_source_ref"}
					return reportReply(out)
				}
			}
		}
	}
	parsed.Mode = rc.NarrativeModel
	out.Narrative = parsed
	return reportReply(out)
}

// --- validate-render --------------------------------------------------------

type reportRenderDocument struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
	Mode     string `json:"mode"`
	Reason   string `json:"reason,omitempty"`
}

type reportRenderOutput struct {
	Pins     reportRunInput       `json:"pins"`
	Document reportRenderDocument `json:"document"`
	Facts    rc.FactBundle        `json:"facts"`
	Provider string               `json:"provider,omitempty"`
	Model    string               `json:"model,omitempty"`
}

func (e *reportNodeExecutor) effectValidateRender(ctx context.Context, call inofy.NodeCall) (inofy.NodeReply, error) {
	in, err := decodeNodeInput[reportNarrateOutput](call)
	if err != nil {
		return reportErrReply(call, inofy.ErrInvalidDefinition, err.Error())
	}
	doc, derr := renderReportDocument(in.Pins, in.Bundle, in.Narrative)
	if derr != nil {
		return reportErrReply(call, inofy.ErrNodeFailed, derr.Error())
	}
	out := reportRenderOutput{
		Pins: in.Pins, Document: doc, Facts: in.Bundle,
		Provider: in.Provider, Model: in.Model,
	}
	return reportReply(out)
}

// renderReportDocument is the deterministic renderer: model claims with
// cited sources become prose; every degraded mode renders factual content
// with its reason disclosed in the body.
func renderReportDocument(pins reportRunInput, bundle rc.FactBundle, narrative rc.Narrative) (reportRenderDocument, error) {
	title := fmt.Sprintf("%s report — %s", strings.Title(string(pins.Period)), pins.WindowID)
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	b.WriteString(fmt.Sprintf("_Window: %s → %s (%s; completed=%v)_\n\n",
		time.UnixMilli(bundle.Window.StartMs).UTC().Format("2006-01-02 15:04"),
		time.UnixMilli(bundle.Window.EndMs).UTC().Format("2006-01-02 15:04"),
		pins.Timezone, bundle.Window.Completed))
	mode := string(narrative.Mode)
	reason := narrative.Reason
	switch narrative.Mode {
	case rc.NarrativeModel, rc.NarrativePartial:
		seen := map[string]bool{}
		var cites []string
		for _, sec := range narrative.Sections {
			b.WriteString("## " + sec.Heading + "\n\n")
			for _, cl := range sec.Claims {
				ref := ""
				if len(cl.SourceIDs) > 0 {
					ref = " (sources: " + strings.Join(cl.SourceIDs, ", ") + ")"
					for _, id := range cl.SourceIDs {
						if !seen[id] {
							seen[id] = true
							cites = append(cites, id)
						}
					}
				}
				b.WriteString("- " + cl.Text + ref + "\n")
			}
			b.WriteString("\n")
		}
		if narrative.Mode == rc.NarrativePartial {
			b.WriteString("_Partial coverage: some sources could not be included._\n\n")
		}
	case rc.NarrativeEmpty:
		b.WriteString("No recorded activity in this window.\n\n")
	default: // fallback
		b.WriteString("_Rendered in fallback mode" + func() string {
			if reason != "" {
				return ": " + reason
			}
			return ""
		}() + "._\n\n")
		b.WriteString("## Facts\n\n")
		for _, f := range bundle.Facts {
			b.WriteString("- [" + f.Date + "] " + f.Text + "\n")
		}
		b.WriteString("\n")
	}
	if len(bundle.MissingDates) > 0 {
		b.WriteString("## Coverage gaps\n\nMissing source days: " + strings.Join(bundle.MissingDates, ", ") + "\n\n")
	}
	if bundle.Excluded > 0 || bundle.Truncated > 0 || bundle.Omitted > 0 {
		b.WriteString(fmt.Sprintf("_Disclosure: %d sources excluded, %d truncated, %d context items omitted._\n\n",
			bundle.Excluded, bundle.Truncated, bundle.Omitted))
	}
	md := b.String()
	if len(md) > reportMaxMarkdownBytes {
		return reportRenderDocument{}, fmt.Errorf("rendered report exceeds %d bytes", reportMaxMarkdownBytes)
	}
	return reportRenderDocument{Title: title, Markdown: md, Mode: mode, Reason: reason}, nil
}

// --- persist ---------------------------------------------------------------

type reportPersistOutput struct {
	Pins       reportRunInput                   `json:"pins"`
	Receipt    storage.ReportPublicationReceipt `json:"receipt"`
	Provenance rc.GenerationProvenance          `json:"provenance"`
}

func (e *reportNodeExecutor) effectPersist(ctx context.Context, call inofy.NodeCall) (inofy.NodeReply, error) {
	in, err := decodeNodeInput[reportRenderOutput](call)
	if err != nil {
		return reportErrReply(call, inofy.ErrInvalidDefinition, err.Error())
	}
	store := e.svc.deps.Report
	if store == nil {
		return reportErrReply(call, inofy.ErrAuthorityDenied, "report publication capability is not configured")
	}
	factsJSON, _ := json.Marshal(in.Facts)
	feedbackJSON, _ := json.Marshal(in.Facts.Feedback)
	gen := storage.ReportGeneration{
		Scope:          in.Pins.Scope,
		RunID:          call.Ref.RunID,
		Period:         string(in.Pins.Period),
		SeriesID:       in.Pins.SeriesID,
		WindowID:       in.Pins.WindowID,
		ConfigRevision: in.Pins.ConfigRevision,
		Timezone:       in.Pins.Timezone,
		WindowStartMs:  in.Pins.WindowStartMs,
		WindowEndMs:    in.Pins.WindowEndMs,
		AsOfMs:         in.Pins.AsOfMs,
		InputDigest:    in.Pins.RequestDigest,
		FactsDigest:    digestJSON("facts", in.Facts),
		Provider:       in.Provider,
		ModelID:        in.Model,
		OutcomeReason:  in.Document.Reason,
		FactsJSON:      factsJSON,
		FeedbackJSON:   feedbackJSON,
	}

	var lastErr error
	for attempt := 0; attempt < reportPersistMaxAttempts; attempt++ {
		// Snapshot the admitted target head inside the same decision the
		// transaction re-verifies: compare target = head the caller saw.
		var admittedHead string
		var admittedVersion int64
		var entryID string
		if in.Pins.Target != nil {
			entryID = in.Pins.Target.EntryID
		}
		head, herr := lookupReportTarget(ctx, store, in.Pins, entryID)
		if herr == nil {
			admittedHead = head.HeadRevisionID
			admittedVersion = head.Version
			entryID = head.EntryID
		} else if !errors.Is(herr, storage.ErrNotFound) {
			return reportErrReply(call, inofy.ErrStorageFailed, herr.Error())
		}
		receipt, perr := store.CommitReportPublication(ctx, storage.ReportPublicationInput{
			Scope:           in.Pins.Scope,
			OperationKey:    "report-persist-" + call.OperationKey,
			RequestDigest:   in.Pins.RequestDigest,
			SectionID:       in.Pins.SectionID,
			EntryID:         entryID,
			SeriesID:        in.Pins.SeriesID,
			WindowID:        in.Pins.WindowID,
			AdmittedHead:    admittedHead,
			AdmittedVersion: admittedVersion,
			Title:           in.Document.Title,
			Markdown:        in.Document.Markdown,
			Actor:           "run:" + call.Ref.RunID,
			Generation:      gen,
		})
		if perr != nil {
			if !isUniqueConflict(perr) || attempt+1 >= reportPersistMaxAttempts {
				return reportErrReply(call, inofy.ErrStorageFailed, perr.Error())
			}
			lastErr = perr
			continue // concurrent first-publication lost the series slot; reread and race as update
		}
		out := reportPersistOutput{
			Pins:    in.Pins,
			Receipt: receipt,
			Provenance: rc.GenerationProvenance{
				RunID: call.Ref.RunID, Scope: in.Pins.Scope, SeriesID: in.Pins.SeriesID,
				WindowID: in.Pins.WindowID, ConfigRevision: in.Pins.ConfigRevision,
				Timezone: in.Pins.Timezone, StartMs: in.Pins.WindowStartMs, EndMs: in.Pins.WindowEndMs,
				AsOfMs: in.Pins.AsOfMs, InputDigest: in.Pins.RequestDigest, FactsDigest: gen.FactsDigest,
				Provider: in.Provider, ModelID: in.Model,
				OutcomeMode: receipt.Outcome, OutcomeReason: in.Document.Reason,
				EntryID: receipt.EntryID, RevisionID: receipt.RevisionID,
			},
		}
		return reportReply(out)
	}
	return reportErrReply(call, inofy.ErrStorageFailed, lastErr.Error())
}

func lookupReportTarget(ctx context.Context, store storage.ReportSourceStore, pins reportRunInput, entryID string) (storage.ReportEntryHead, error) {
	if entryID != "" {
		return store.GetReportEntryHead(ctx, pins.Scope, entryID)
	}
	return store.FindReportEntry(ctx, pins.Scope, pins.SeriesID, pins.WindowID)
}

func isUniqueConflict(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate key"))
}

func reportErrReply(call inofy.NodeCall, code inofy.ErrorCode, msg string) (inofy.NodeReply, error) {
	return inofy.NodeReply{}, &inofy.Error{Code: code, Path: call.Path, Message: msg}
}
