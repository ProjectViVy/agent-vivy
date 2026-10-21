package runtime

import (
	"errors"
)

// Tool-loop detection (VC-2; behavior aligned with Crush's StopWhen):
// when the same tool call — same tool, same arguments, same result —
// repeats more than loopRepeatLimit times inside the last
// loopWindowSteps completed calls, the run is stopped. A model stuck
// repeating itself burns budget without making progress.
const (
	loopWindowSteps = 10
	loopRepeatLimit = 5
)

// errLoopDetected is the runtime sentinel for a detected tool loop; the
// service turns it into a bounded, classified run.failed (FR-11).
var errLoopDetected = errors.New("runtime: tool loop detected")

// loopWindow compares the bounded completed values without serialization.
// Old entries are overwritten so private strings live for at most ten calls.
type loopWindow struct {
	calls [][4]string
}

// record appends one completed call tuple and returns how many
// identical tuples the window now holds, plus errLoopDetected once
// the repetition limit is exceeded. argsJSON must already be canonical.
func (w *loopWindow) record(toolName, argsJSON, result, errMsg string) (int, error) {
	call := [4]string{toolName, argsJSON, result, errMsg}
	if len(w.calls) == loopWindowSteps {
		copy(w.calls, w.calls[1:])
		w.calls[len(w.calls)-1] = call
	} else {
		w.calls = append(w.calls, call)
	}
	count := 0
	for _, prior := range w.calls {
		if prior == call {
			count++
		}
	}
	if count > loopRepeatLimit {
		return count, errLoopDetected
	}
	return count, nil
}
