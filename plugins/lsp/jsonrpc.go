package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// rpcMessage is one JSON-RPC 2.0 message on the LSP wire. A message is
// exactly one of: request (ID + Method), response (ID + Result/Error), or
// notification (Method, no ID).
type rpcMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

func (m rpcMessage) isNotification() bool { return m.ID == nil && m.Method != "" }
func (m rpcMessage) isResponse() bool     { return m.ID != nil && m.Method == "" }

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("lsp: rpc error %d: %s", e.Code, e.Message)
}

// writeMessage frames one message with the LSP base protocol
// (Content-Length header, CRLF, blank line, UTF-8 body).
func writeMessage(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("lsp: encode: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return fmt.Errorf("lsp: write header: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("lsp: write body: %w", err)
	}
	return nil
}

// readMessage reads one framed message. Unknown header fields (servers
// send Content-Type) are skipped.
func readMessage(r *bufio.Reader) (rpcMessage, error) {
	length := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return rpcMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return rpcMessage{}, fmt.Errorf("lsp: bad Content-Length: %w", err)
			}
		}
	}
	if length <= 0 {
		return rpcMessage{}, fmt.Errorf("lsp: missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return rpcMessage{}, fmt.Errorf("lsp: read body: %w", err)
	}
	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return rpcMessage{}, fmt.Errorf("lsp: bad body: %w", err)
	}
	return msg, nil
}
