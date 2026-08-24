//go:build vivy_headless

package ui

import "net/http"

// Handler keeps the app composition identical while leaving static assets out
// of the backend build. The control plane remains available under /rpc.
func Handler() http.Handler {
	return http.NotFoundHandler()
}
