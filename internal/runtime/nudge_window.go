package runtime

import "hash/fnv"

const nudgeWindowSteps = 10

type nudgeWindow struct {
	sigs []uint64
}

func (w *nudgeWindow) record(toolName, argsJSON, result, errMsg string) int {
	h := fnv.New64a()
	for _, part := range []string{toolName, argsJSON, result, errMsg} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	sig := h.Sum64()
	w.sigs = append(w.sigs, sig)
	if len(w.sigs) > nudgeWindowSteps {
		w.sigs = w.sigs[len(w.sigs)-nudgeWindowSteps:]
	}
	count := 0
	for _, previous := range w.sigs {
		if previous == sig {
			count++
		}
	}
	return count
}
