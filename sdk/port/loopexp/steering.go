package loopexp

// SteeringFactory constructs policy without queue, Run or checkpoint authority.
type SteeringFactory func() SteeringPolicy

type SteeringEligibility struct {
	Suspended, HasBoundaryCancel, PreferencesCompatible bool
	HasAttachments, HasFileContexts, HasContinuity      bool
}

// QueueTrack projects the existing steer/follow_up owner value.
type QueueTrack string

// QueueMode projects the existing all/one-at-a-time owner value.
type QueueMode string

type SteeringPolicy interface {
	Route(SteeringEligibility) QueueTrack
	Take(QueueMode, int) int
	Render(selectedTexts []string) string
}
