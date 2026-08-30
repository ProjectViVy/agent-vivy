// Package tui is the first-cut terminal face: a JSON-RPC control-plane
// client, not a second run loop. It talks to a resident vivy process
// (the web gateway) over the same /rpc WebSocket the browser uses.
//
// Modes:
//   - plain REPL (RunREPL)
//   - fullscreen Crush-style shell on offline demo data (view + demo)
//   - fullscreen shell on a live Client (Live + view)
//
// This is not the packed faces/tui organ in VIVY-FACE-PACK.md. That
// generation still requires FaceHost and a recipe that omits web.
package tui
