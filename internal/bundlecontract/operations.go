package bundlecontract

// Operation is durable management evidence, independent from UI request state.
type Operation struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	State          string    `json:"state"`
	Revision       int64     `json:"revision"`
	ExpectedEpoch  int64     `json:"expected_epoch"`
	CommittedEpoch int64     `json:"committed_epoch"`
	ErrorCode      string    `json:"error_code,omitempty"`
	Retryable      bool      `json:"retryable"`
	Progress       string    `json:"progress,omitempty"`
	Blockers       []Blocker `json:"blockers"`
}
type Blocker struct {
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}
