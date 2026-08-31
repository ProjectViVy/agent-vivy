package runtime

import (
	"errors"
	"hash/fnv"
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

// errLoopDetected is the mapper sentinel for a detected tool loop; the
// service turns it into a bounded, classified run.failed (FR-11).
var errLoopDetected = errors.New("runtime: tool loop detected")

// loopWindow keeps the signatures of the most recent completed tool
// calls. Calls are keyed by hash so results never linger in memory and
// comparison stays O(window).
type loopWindow struct {
	sigs []uint64
}

// record appends one completed call signature and reports whether the
// repetition limit is exceeded. argsJSON must already be canonical.
func (w *loopWindow) record(toolName, argsJSON, result, errMsg string) error {
	h := fnv.New64a()
	for _, part := range []string{toolName, argsJSON, result, errMsg} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	sig := h.Sum64()
	w.sigs = append(w.sigs, sig)
	if len(w.sigs) > loopWindowSteps {
		w.sigs = w.sigs[len(w.sigs)-loopWindowSteps:]
	}
	count := 0
	for _, s := range w.sigs {
		if s == sig {
			count++
		}
	}
	if count > loopRepeatLimit {
		return errLoopDetected
	}
	return nil
}
