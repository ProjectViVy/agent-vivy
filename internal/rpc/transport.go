package rpc

import (
	"bufio"
	"errors"
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

type JSONLTransport struct {
	reader *bufio.Reader
	writer io.Writer
	flush  interface{ Flush() error }
	close  func() error
	mu     sync.Mutex
}

func NewJSONLTransport(reader io.Reader, writer io.Writer, close func() error) *JSONLTransport {
	if close == nil {
		close = func() error { return nil }
	}
	var flush interface{ Flush() error }
	if f, ok := writer.(interface{ Flush() error }); ok {
		flush = f
	}
	return &JSONLTransport{reader: bufio.NewReader(reader), writer: writer, flush: flush, close: close}
}

func (t *JSONLTransport) ReadFrame() ([]byte, error) {
	frame, err := t.reader.ReadBytes('\n')
	if len(frame) > 0 {
		for len(frame) > 0 && (frame[len(frame)-1] == '\n' || frame[len(frame)-1] == '\r') {
			frame = frame[:len(frame)-1]
		}
		if len(frame) > 0 {
			return frame, nil
		}
	}
	return nil, err
}

func (t *JSONLTransport) WriteFrame(frame []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, err := t.writer.Write(append(frame, '\n')); err != nil {
		return err
	}
	if t.flush != nil {
		return t.flush.Flush()
	}
	return nil
}

func (t *JSONLTransport) Close() error { return t.close() }

type WebSocketTransport struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewWebSocketTransport(conn *websocket.Conn) *WebSocketTransport {
	return &WebSocketTransport{conn: conn}
}

func (t *WebSocketTransport) ReadFrame() ([]byte, error) {
	messageType, frame, err := t.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if messageType != websocket.TextMessage {
		return nil, errors.New("rpc: websocket frame must be text")
	}
	return frame, nil
}

func (t *WebSocketTransport) WriteFrame(frame []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, frame)
}

func (t *WebSocketTransport) Close() error { return t.conn.Close() }
