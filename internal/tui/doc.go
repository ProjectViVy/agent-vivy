// Package tui is the first-cut terminal face: a JSON-RPC control-plane
// client, not a second run loop. It talks to a resident vivy process
// (the web gateway) over the same /rpc WebSocket the browser uses.
//
// This is not the packed faces/tui organ in VIVY-FACE-PACK.md. That
// generation still requires FaceHost and a recipe that omits web. The
// command here is the smallest mouth that can sit on a TTY without
// importing the kernel or embedding Bubble Tea.
package tui
