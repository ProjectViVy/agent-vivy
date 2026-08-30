package domain

// CronJob is one scheduled task in the control plane's CRON panel. The
// zero-value semantics follow the repo's unix-milli convention: state
// timestamps of 0 mean "never / not scheduled" (diva uses Option there).
// JSON tags back the durable schedule_json/payload_json columns shared by
// both storage backends; the RPC layer maps these onto its own wire DTO.
type CronJob struct {
	ID       string
	Name     string
	Enabled  bool
	Schedule CronSchedule
	Payload  CronPayload
	// SessionID is the job's dedicated conversation (Vivy extension over
	// diva): fired turns run here so history stays inspectable. Empty
	// means the session is created lazily on first fire.
	SessionID      SessionID
	State          CronJobState
	DeleteAfterRun bool
	CreatedAt      int64
	UpdatedAt      int64
}

type CronScheduleKind string

const (
	CronScheduleAt    CronScheduleKind = "at"
	CronScheduleEvery CronScheduleKind = "every"
	CronScheduleCron  CronScheduleKind = "cron"
)

// Valid reports whether the kind is one of the three schedule forms.
func (k CronScheduleKind) Valid() bool {
	switch k {
	case CronScheduleAt, CronScheduleEvery, CronScheduleCron:
		return true
	}
	return false
}

type CronSchedule struct {
	Kind    CronScheduleKind `json:"kind"`
	AtMs    int64            `json:"atMs,omitempty"`
	EveryMs int64            `json:"everyMs,omitempty"`
	Expr    string           `json:"expr,omitempty"`
	TZ      string           `json:"tz,omitempty"`
}

type CronPayload struct {
	// Kind is reserved for forward compatibility; the only behavior the
	// scheduler implements today is the agent turn below (diva's
	// agent_turn). Other values are stored but never fire.
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Deliver bool   `json:"deliver"`
	Channel string `json:"channel,omitempty"`
	To      string `json:"to,omitempty"`
}

// CronPayloadKindAgentTurn is the one payload behavior the scheduler
// implements: the message drives one run in the job's session.
const CronPayloadKindAgentTurn = "agent_turn"

type CronJobState struct {
	NextRunAtMs int64
	LastRunAtMs int64
	// LastStatus is "", "ok" or "error" (diva's two-value terminal set;
	// cancelled runs land on "error" with LastError explaining why).
	LastStatus string
	LastError  string
}
