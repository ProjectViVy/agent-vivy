package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"

	plugin "agent-vivy/sdk/port/channel"
)

const dingtalkDefaultOpenAPIHost = "https://api.dingtalk.com"

// governedStreamClient implements the small DingTalk Stream protocol surface
// the adapter needs. The upstream SDK hard-codes its HTTP transport and
// websocket dialer, so this client routes both operations through the Host.
type governedStreamClient struct {
	mu       sync.Mutex
	host     plugin.ChannelEnv
	creds    streamCreds
	baseURL  string
	handler  chatbot.IChatBotMessageHandler
	conn     *websocket.Conn
	readDone <-chan struct{}
	closing  bool
}

func newGovernedStreamClient(host plugin.ChannelEnv, creds streamCreds, baseURL string) *governedStreamClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = dingtalkDefaultOpenAPIHost
	}
	return &governedStreamClient{host: host, creds: creds, baseURL: strings.TrimRight(baseURL, "/")}
}

func (client *governedStreamClient) RegisterChatBotCallbackRouter(handler chatbot.IChatBotMessageHandler) {
	client.mu.Lock()
	client.handler = handler
	client.mu.Unlock()
}

func (client *governedStreamClient) Start(ctx context.Context) error {
	client.mu.Lock()
	if client.conn != nil {
		client.mu.Unlock()
		return nil
	}
	client.closing = false
	client.mu.Unlock()
	if client.host == nil {
		return errors.New("dingtalk: network host is not bound")
	}

	endpoint, err := client.endpoint(ctx)
	if err != nil {
		return err
	}
	wsURL, err := url.Parse(endpoint.Endpoint)
	if err != nil {
		return fmt.Errorf("parse stream endpoint: %w", err)
	}
	query := wsURL.Query()
	query.Set("ticket", endpoint.Ticket)
	wsURL.RawQuery = query.Encode()
	dialer := *websocket.DefaultDialer
	dialer.Proxy = nil
	dialer.NetDialContext = func(context.Context, string, string) (net.Conn, error) { return nil, plugin.ErrDenied }
	dialer.NetDialTLSContext = client.host.DialTLS
	conn, response, err := dialer.DialContext(ctx, wsURL.String(), nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("dial stream endpoint: %w", err)
	}
	client.mu.Lock()
	if client.closing {
		client.mu.Unlock()
		_ = conn.Close()
		return context.Canceled
	}
	done := make(chan struct{})
	client.conn = conn
	client.readDone = done
	client.mu.Unlock()
	go client.readLoop(ctx, conn, done)
	return nil
}

func (client *governedStreamClient) endpoint(ctx context.Context) (*payload.ConnectionEndpointResponse, error) {
	body, err := json.Marshal(payload.ConnectionEndpointRequest{
		ClientId:     client.creds.ClientID,
		ClientSecret: client.creds.ClientSecret,
		UserAgent:    "agent-vivy/dingtalk",
		Subscriptions: []*payload.SubscriptionModel{
			{Type: "CALLBACK", Topic: payload.BotMessageCallbackTopic},
			{Type: "SYSTEM", Topic: "disconnect"},
			{Type: "SYSTEM", Topic: "ping"},
		},
	})
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, client.baseURL+"/v1.0/gateway/connections/open", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.host.HTTP().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("stream endpoint returned HTTP %d", response.StatusCode)
	}
	var endpoint payload.ConnectionEndpointResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&endpoint); err != nil {
		return nil, err
	}
	if err := endpoint.Valid(); err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func (client *governedStreamClient) readLoop(ctx context.Context, conn *websocket.Conn, done chan<- struct{}) {
	defer func() {
		client.mu.Lock()
		if client.conn == conn {
			client.conn = nil
		}
		client.mu.Unlock()
		_ = conn.Close()
		close(done)
	}()
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := payload.DecodeDataFrame(raw)
		if err != nil || frame.Headers == nil {
			continue
		}
		if frame.Type == "SYSTEM" && frame.GetTopic() == "disconnect" {
			return
		}
		response := payload.NewSuccessDataFrameResponse()
		if frame.Type == "SYSTEM" && frame.GetTopic() == "ping" {
			response = payload.NewDataFrameAckPong(frame.GetMessageId())
		} else if frame.Type == "CALLBACK" && frame.GetTopic() == payload.BotMessageCallbackTopic {
			client.mu.Lock()
			handler := client.handler
			client.mu.Unlock()
			if handler == nil {
				response = payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound)
			} else {
				var callback chatbot.BotCallbackDataModel
				if err := json.Unmarshal([]byte(frame.Data), &callback); err != nil {
					response = payload.NewErrorDataFrameResponse(err)
				} else if data, err := handler(ctx, &callback); err != nil {
					response = payload.NewErrorDataFrameResponse(err)
				} else {
					response.SetData(string(data))
				}
			}
		} else {
			response = payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound)
		}
		response.SetHeader(payload.DataFrameHeaderKMessageId, frame.GetMessageId())
		response.SetHeader(payload.DataFrameHeaderKContentType, payload.DataFrameContentTypeKJson)
		if err := conn.WriteJSON(response); err != nil {
			return
		}
	}
}

func (client *governedStreamClient) Close() {
	client.mu.Lock()
	client.closing = true
	conn := client.conn
	client.conn = nil
	client.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Wait blocks until the current socket reader has exited. Close is kept
// non-blocking because a Channel callback may itself trigger shutdown.
func (client *governedStreamClient) Wait(ctx context.Context) error {
	client.mu.Lock()
	done := client.readDone
	client.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
