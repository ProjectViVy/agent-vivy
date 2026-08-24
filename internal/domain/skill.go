package domain

// SkillRevisionStatus is the durable lifecycle of a staged Skill mutation.
type SkillRevisionStatus string

const (
	SkillRevisionPending  SkillRevisionStatus = "pending"
	SkillRevisionApplied  SkillRevisionStatus = "applied"
	SkillRevisionRejected SkillRevisionStatus = "rejected"
	SkillRevisionFailed   SkillRevisionStatus = "failed"
)

// SkillRevision is a reviewable, restart-safe Skill change. Payload is an
// opaque JSON mutation document owned by the Skill backend; it is never
// interpreted by storage.
type SkillRevision struct {
	ID            string
	RunID         RunID
	SkillName     string
	Action        string
	TargetPath    string
	Payload       []byte
	BeforePayload []byte
	BaseHash      string
	ContentHash   string
	Preview       string
	WarningsJSON  []byte
	Status        SkillRevisionStatus
	CreatedAt     int64
	AppliedAt     int64
}
