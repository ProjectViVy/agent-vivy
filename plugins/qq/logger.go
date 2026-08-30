package qq

import (
	"fmt"
	"os"
)

// quietLogger is the botgo SDK logger this adapter installs. botgo's
// default console logger prints every websocket frame and every OpenAPI
// request/response body at INFO level — including the identify payload
// (which carries the access token) and raw event payloads (which carry
// user message content). That violates the kernel logging contract
// (docs/architecture/LOGGING.md) and D-010: no tokens or message content
// in logs, and the kernel owns all structured log output. A plugin cannot
// import internal/logging, so the SDK logger is muted except for errors,
// which reach stderr without payload bodies.
type quietLogger struct{}

// The botgo log.Logger interface. Debug/Info/Warn carry frame dumps,
// heartbeat traffic and request bodies — all discarded.
func (quietLogger) Debug(v ...interface{})                 {}
func (quietLogger) Info(v ...interface{})                  {}
func (quietLogger) Warn(v ...interface{})                  {}
func (quietLogger) Debugf(format string, v ...interface{}) {}
func (quietLogger) Infof(format string, v ...interface{})  {}
func (quietLogger) Warnf(format string, v ...interface{})  {}

// Error and Errorf keep the SDK's failure paths visible on stderr. botgo
// error lines carry session coordinates and close codes, not payloads —
// the two error sites that could echo a raw frame (a failed websocket
// read) only fire on an already-broken connection.
func (quietLogger) Error(v ...interface{}) {
	fmt.Fprintln(os.Stderr, append([]interface{}{"[qq/botgo] "}, v...)...)
}

func (quietLogger) Errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "[qq/botgo] "+format+"\n", v...)
}

// Sync is a no-op: stderr is unbuffered.
func (quietLogger) Sync() error { return nil }
