package runtime

import (
	"errors"
	"strings"
)

// ErrLegacyOrchestrationResumeUnsupported marks approvals that point into the
// retired native orchestration proof. The approval remains pending so its
// historical record can be inspected without executing obsolete graph code.
var ErrLegacyOrchestrationResumeUnsupported = errors.New("runtime: legacy orchestration resume is unsupported")

const nativeOrchestrationResumeTargetPrefix = "native-orchestration:"

func isNativeOrchestrationResumeTarget(target string) bool {
	return strings.HasPrefix(target, nativeOrchestrationResumeTargetPrefix)
}
