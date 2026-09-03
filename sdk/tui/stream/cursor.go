package stream

// Cursor enforces exactly-once, contiguous application of durable run events.
// Sequence zero remains accepted for old transports and unit-level adapters
// that do not carry a journal cursor.
type Cursor struct {
	LastSeq    int
	Recovering bool
}

// Reset starts a new run cursor.
func (c *Cursor) Reset() {
	c.LastSeq = 0
	c.Recovering = false
}

// Accept reports whether notice should be reduced and whether a replay from
// LastSeq is required. Unknown notices are accepted for cursor advancement;
// callers may still skip their rendering when Kind is empty.
func (c *Cursor) Accept(activeRunID string, notice Notice) (accept bool, gap bool) {
	if notice.Seq <= 0 {
		return true, false
	}
	// A numbered event belongs to exactly one active run. In particular,
	// reject late replay after terminal state has cleared activeRunID.
	if notice.RunID != "" && notice.RunID != activeRunID {
		return false, false
	}
	if notice.Seq <= c.LastSeq {
		return false, false
	}
	if notice.Seq != c.LastSeq+1 {
		c.Recovering = true
		return false, true
	}
	c.LastSeq = notice.Seq
	c.Recovering = false
	return true, false
}
