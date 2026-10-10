package runtime

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/sdk/port/contextsource"
)

func TestContextSourceAdmissionCannotBeForgedByRequest(t *testing.T) {
	request := contextsource.Request{TenantID: "tenant", SessionID: "session", WorkspaceID: "workspace"}
	if err := AuthorizeContextSourceRequest(context.Background(), request); !errors.Is(err, ErrContextSourceUnauthorized) {
		t.Fatalf("request fields alone authorized memory: %v", err)
	}
	ctx := withContextSourceAdmission(context.Background(), request.TenantID, request.SessionID, request.WorkspaceID)
	if err := AuthorizeContextSourceRequest(ctx, request); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant", "session", "workspace"} {
		t.Run(field, func(t *testing.T) {
			changed := request
			switch field {
			case "tenant":
				changed.TenantID = "foreign"
			case "session":
				changed.SessionID = "foreign"
			case "workspace":
				changed.WorkspaceID = "foreign"
			}
			if err := AuthorizeContextSourceRequest(ctx, changed); !errors.Is(err, ErrContextSourceUnauthorized) {
				t.Fatalf("foreign %s authorized: %v", field, err)
			}
		})
	}
}
