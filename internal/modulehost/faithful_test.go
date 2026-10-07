package modulehost

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

type failingSecretReader struct{ cause error }

func (reader failingSecretReader) ReadSecret(context.Context, string) (string, error) {
	return "", reader.cause
}

func TestAuthorizedModuleErrorPreservesCauseAndText(t *testing.T) {
	cause := errors.New("alice@example.com password=synthetic [REDACTED]")
	host, err := New(Config{ModuleID: "example/test", InstanceID: "fixture", Secrets: failingSecretReader{cause}, Grants: []module.GrantBinding{{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"fixture"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Secret(context.Background(), "fixture")
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("authorized error changed: %v", err)
	}
}
