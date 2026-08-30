package badchannellisten2

import (
	"net/http"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

// listenProbe is never called; it exists so the method-form AST ban catches
// a channel plugin opening a listen socket through a server value instead
// of the package-level helpers (review L4: &http.Server{}.
// ListenAndServe bypasses a package-name-only check).
var _ = func() error {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	return srv.ListenAndServe()
}
