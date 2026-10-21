// Package loopexp defines advisory loop policy values without execution authority.
package loopexp

type NudgeFactory func() NudgeDetector

type ToolSample struct {
	CallID, ToolName         string
	Arguments, Result, Error string
	Status, Reason, Effects  string
}

type NudgeNotice struct {
	CallID, ToolName, Status, Reason string
	Count                            int
	TemplateVersion, Text            string
	// Stop carries the existing loop-limit decision at the later extraction boundary.
	Stop bool
}

type NudgeDetector interface {
	Evaluate(orderedFresh []ToolSample) (NudgeNotice, bool)
}
