package turnlimit

import "errors"

const defaultMaxToolTurns = 8

type Policy struct {
	MaxToolTurns int `yaml:"max_tool_turns"`
}

func Default() Policy {
	return Policy{MaxToolTurns: defaultMaxToolTurns}
}

func (p Policy) MaxIterations(child bool) (int, error) {
	if p.MaxToolTurns < 0 {
		return 0, errors.New("runtime.max_tool_turns must not be negative")
	}
	if child && (p.MaxToolTurns == 0 || p.MaxToolTurns > defaultMaxToolTurns) {
		return defaultMaxToolTurns, nil
	}
	return p.MaxToolTurns, nil
}
