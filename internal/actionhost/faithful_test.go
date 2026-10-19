package actionhost

import (
	"errors"
	"testing"

	action "agent-vivy/sdk/port/controlaction"
)

func TestAuditErrorPreservesTaskTextWhenResolvedSecretIsAbsent(t *testing.T) {
	host := &Host{}
	text := "alice@example.com password=synthetic [REDACTED]"
	if got := host.safeError(errors.New(text), []string{"resolver-issued-value"}); got != text {
		t.Fatalf("authorized diagnostic changed: %q", got)
	}
	if got := host.safeError(errors.New("failed with resolver-issued-value"), []string{"resolver-issued-value"}); got != action.ErrSecretLeak.Error() {
		t.Fatalf("actual Secret authority boundary lost: %q", got)
	}
}
