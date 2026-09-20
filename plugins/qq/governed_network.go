package qq

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
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/errs"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi/options"
	"golang.org/x/oauth2"

	plugin "agent-vivy/sdk/port/channel"
)

// Governed network endpoints. Vars so the loopback media test can pin
// them to a local server; production never mutates them.
var (
	qqTokenURL       = "https://bots.qq.com/app/getAppAccessToken"
	qqAPIBaseURL     = "https://api.sgroup.qq.com"
	qqSandboxBaseURL = "https://sandbox.api.sgroup.qq.com"
)

type governedQQTokenSource struct {
	mu     sync.Mutex
	ctx    context.Context
	host   plugin.ChannelEnv
	appID  string
	secret string
	cached *oauth2.Token
}

func newGovernedQQTokenSource(ctx context.Context, host plugin.ChannelEnv, appID, secret string) oauth2.TokenSource {
	return &governedQQTokenSource{ctx: ctx, host: host, appID: appID, secret: secret}
}

func (source *governedQQTokenSource) Token() (*oauth2.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.cached != nil && source.cached.Valid() {
		copy := *source.cached
		return &copy, nil
	}
	if source.host == nil {
		return nil, errors.New("qq: network host is not bound")
	}
	body, err := json.Marshal(map[string]string{"appId": source.appID, "clientSecret": source.secret})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(source.ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, qqTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := source.host.HTTP().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result struct {
		Code        int             `json:"code"`
		Message     string          `json:"message"`
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result); err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 || result.AccessToken == "" {
		// The platform message is untrusted and may echo the submitted app
		// secret. Status and numeric code are sufficient diagnostics.
		return nil, fmt.Errorf("token endpoint rejected credentials (http=%d code=%d)", response.StatusCode, result.Code)
	}
	var seconds int64
	if err := json.Unmarshal(result.ExpiresIn, &seconds); err != nil {
		var text string
		if err := json.Unmarshal(result.ExpiresIn, &text); err != nil {
			return nil, errors.New("qq: token endpoint returned invalid expiry")
		}
		if _, err := fmt.Sscan(text, &seconds); err != nil {
			return nil, errors.New("qq: token endpoint returned invalid expiry")
		}
	}
	token := &oauth2.Token{AccessToken: result.AccessToken, TokenType: "QQBot", Expiry: time.Now().Add(time.Duration(seconds) * time.Second)}
	source.cached = token
	copy := *token
	return &copy, nil
}

type governedQQAPI struct {
	host        plugin.ChannelEnv
	appID       string
	tokenSource oauth2.TokenSource
	baseURL     string
}

func newGovernedQQAPI(host plugin.ChannelEnv, appID string, tokenSource oauth2.TokenSource, sandbox bool) qqAPI {
	baseURL := qqAPIBaseURL
	if sandbox {
		baseURL = qqSandboxBaseURL
	}
	return &governedQQAPI{host: host, appID: appID, tokenSource: tokenSource, baseURL: baseURL}
}

func (api *governedQQAPI) WS(ctx context.Context, _ map[string]string, _ string) (*dto.WebsocketAP, error) {
	var result dto.WebsocketAP
	if err := api.do(ctx, http.MethodGet, "/gateway/bot", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *governedQQAPI) PostC2CMessage(ctx context.Context, userID string, message dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	var result dto.Message
	path := "/v2/users/" + url.PathEscape(userID) + "/messages"
	if err := api.do(ctx, http.MethodPost, path, message, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PostGroupMessage posts one message to a group
// (POST /v2/groups/{group_openid}/messages) — the same MessageToCreate
// contract as the C2C endpoint, addressed by the group_openid a group AT
// event carries.
func (api *governedQQAPI) PostGroupMessage(ctx context.Context, groupOpenID string, message dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	var result dto.Message
	path := "/v2/groups/" + url.PathEscape(groupOpenID) + "/messages"
	if err := api.do(ctx, http.MethodPost, path, message, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *governedQQAPI) do(ctx context.Context, method, path string, body any, result any) error {
	if api.host == nil {
		return errors.New("qq: network host is not bound")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, api.baseURL+path, reader)
	if err != nil {
		return err
	}
	token, err := api.tokenSource.Token()
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", strings.TrimSpace(token.TokenType+" "+token.AccessToken))
	request.Header.Set("X-Union-Appid", api.appID)
	request.Header.Set("Content-Type", "application/json")
	response, err := api.host.HTTP().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &failure)
		return fmt.Errorf("QQ API returned HTTP %d (code=%d message=%s)", response.StatusCode, failure.Code, failure.Message)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, result)
}

// qqMediaUpload is the official v2 file-upload body
// (POST /v2/{users|groups}/{id}/files). FileType: 1 image, 2 video,
// 3 voice, 4 file — this adapter only ever sends 1 (the §12 ruling scopes
// media to images). FileData carries the base64 bytes, SrvSendMsg=false
// keeps the upload passive (bound to the next rich-media message).
type qqMediaUpload struct {
	FileType   int    `json:"file_type"`
	URL        string `json:"url,omitempty"`
	FileData   string `json:"file_data,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	SrvSendMsg bool   `json:"srv_send_msg,omitempty"`
}

// qqRichMediaMessage is the msg_type=7 reply body with the uploaded file's
// handle. Plugin-owned on purpose: botgo's MessageToCreate types MediaInfo
// as []byte, whose JSON marshaling would base64 the file_info string a
// second time.
type qqRichMediaMessage struct {
	MsgType int `json:"msg_type"`
	Media   struct {
		FileInfo string `json:"file_info"`
	} `json:"media"`
	MsgID  string `json:"msg_id"`
	MsgSeq uint32 `json:"msg_seq"`
}

func (api *governedQQAPI) PostC2CMediaUpload(ctx context.Context, userID string, upload qqMediaUpload) (string, error) {
	return api.mediaUpload(ctx, "/v2/users/"+url.PathEscape(userID)+"/files", upload)
}

func (api *governedQQAPI) PostGroupMediaUpload(ctx context.Context, groupOpenID string, upload qqMediaUpload) (string, error) {
	return api.mediaUpload(ctx, "/v2/groups/"+url.PathEscape(groupOpenID)+"/files", upload)
}

func (api *governedQQAPI) mediaUpload(ctx context.Context, path string, upload qqMediaUpload) (string, error) {
	var result struct {
		FileInfo string `json:"file_info"`
	}
	if err := api.do(ctx, http.MethodPost, path, upload, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.FileInfo) == "" {
		return "", errors.New("qq: media upload returned no file_info")
	}
	return result.FileInfo, nil
}

func (api *governedQQAPI) PostC2CRichMedia(ctx context.Context, userID string, msg qqRichMediaMessage) (*dto.Message, error) {
	var result dto.Message
	path := "/v2/users/" + url.PathEscape(userID) + "/messages"
	if err := api.do(ctx, http.MethodPost, path, msg, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *governedQQAPI) PostGroupRichMedia(ctx context.Context, groupOpenID string, msg qqRichMediaMessage) (*dto.Message, error) {
	var result dto.Message
	path := "/v2/groups/" + url.PathEscape(groupOpenID) + "/messages"
	if err := api.do(ctx, http.MethodPost, path, msg, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type governedQQWebSocket struct {
	mu        sync.Mutex
	writeMu   sync.Mutex
	sessionMu sync.Mutex
	host      plugin.ChannelEnv
	session   *dto.Session
	onC2C     event.C2CMessageEventHandler
	onGroup   groupATMessageHandler
	onReady   event.ReadyHandler
	conn      *websocket.Conn
}

func newGovernedQQWebSocket(host plugin.ChannelEnv, session dto.Session, onC2C event.C2CMessageEventHandler, onGroup groupATMessageHandler, onReady event.ReadyHandler) wsClient {
	return &governedQQWebSocket{host: host, session: &session, onC2C: onC2C, onGroup: onGroup, onReady: onReady}
}

func (client *governedQQWebSocket) Connect() error {
	if client.host == nil {
		return errors.New("qq: network host is not bound")
	}
	if strings.TrimSpace(client.session.URL) == "" {
		return errors.New("qq: websocket URL is empty")
	}
	dialer := *websocket.DefaultDialer
	dialer.Proxy = nil
	dialer.NetDialContext = func(context.Context, string, string) (net.Conn, error) { return nil, plugin.ErrDenied }
	dialer.NetDialTLSContext = client.host.DialTLS
	conn, response, err := dialer.Dial(client.session.URL, nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return err
	}
	client.mu.Lock()
	client.conn = conn
	client.mu.Unlock()
	return nil
}

func (client *governedQQWebSocket) Identify() error {
	token, err := client.session.TokenSource.Token()
	if err != nil {
		return err
	}
	return client.write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSIdentity}, Data: &dto.WSIdentityData{
		Token:   token.TokenType + " " + token.AccessToken,
		Intents: client.session.Intent,
		Shard:   []uint32{client.session.Shards.ShardID, client.session.Shards.ShardCount},
	}})
}

func (client *governedQQWebSocket) Resume() error {
	token, err := client.session.TokenSource.Token()
	if err != nil {
		return err
	}
	client.sessionMu.Lock()
	sessionID, lastSeq := client.session.ID, client.session.LastSeq
	client.sessionMu.Unlock()
	return client.write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSResume}, Data: &dto.WSResumeData{
		Token: token.AccessToken, SessionID: sessionID, Seq: lastSeq,
	}})
}

func (client *governedQQWebSocket) Session() *dto.Session { return client.session }

func (client *governedQQWebSocket) Listening() error {
	client.mu.Lock()
	conn := client.conn
	client.mu.Unlock()
	if conn == nil {
		return errors.New("qq: websocket is not connected")
	}
	done := make(chan struct{})
	defer close(done)
	var heartbeatOnce sync.Once
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var envelope struct {
			OPCode dto.OPCode      `json:"op"`
			Seq    uint32          `json:"s"`
			Type   dto.EventType   `json:"t"`
			ID     string          `json:"id"`
			Data   json.RawMessage `json:"d"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		payload := &dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: envelope.OPCode, Seq: envelope.Seq, Type: envelope.Type, EventID: envelope.ID}, RawMessage: raw, Session: client.session}
		if envelope.Seq > 0 {
			client.sessionMu.Lock()
			client.session.LastSeq = envelope.Seq
			client.sessionMu.Unlock()
		}
		switch envelope.OPCode {
		case dto.WSHello:
			var hello dto.WSHelloData
			if json.Unmarshal(envelope.Data, &hello) == nil && hello.HeartbeatInterval > 0 {
				heartbeatOnce.Do(func() { go client.heartbeat(done, time.Duration(hello.HeartbeatInterval)*time.Millisecond) })
			}
		case dto.WSHeartbeatAck:
		case dto.WSReconnect:
			return errs.ErrNeedReConnect
		case dto.WSInvalidSession:
			return errs.ErrInvalidSession
		case dto.WSDispatchEvent:
			switch envelope.Type {
			case "READY":
				var ready dto.WSReadyData
				if err := json.Unmarshal(envelope.Data, &ready); err != nil {
					return err
				}
				client.sessionMu.Lock()
				client.session.ID = ready.SessionID
				if len(ready.Shard) >= 2 {
					client.session.Shards = dto.ShardConfig{ShardID: ready.Shard[0], ShardCount: ready.Shard[1]}
				}
				client.sessionMu.Unlock()
				if client.onReady != nil {
					client.onReady(payload, &ready)
				}
			case dto.EventC2CMessageCreate:
				var message dto.WSC2CMessageData
				if err := json.Unmarshal(envelope.Data, &message); err != nil {
					return err
				}
				if client.onC2C != nil {
					if err := client.onC2C(payload, &message); err != nil {
						return err
					}
				}
			case dto.EventGroupAtMessageCreate:
				// Decoded into the plugin's own struct: the pinned botgo
				// dto.Message reads a group_id field the real v2 group
				// payload never sends (it carries group_openid), so the
				// SDK's own dispatcher cannot address a group.
				var group groupATMessage
				if err := json.Unmarshal(envelope.Data, &group); err != nil {
					return err
				}
				if client.onGroup != nil {
					if err := client.onGroup(payload, &group); err != nil {
						return err
					}
				}
			}
		}
	}
}

func (client *governedQQWebSocket) heartbeat(done <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			client.sessionMu.Lock()
			lastSeq := client.session.LastSeq
			client.sessionMu.Unlock()
			_ = client.write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSHeartbeat}, Data: lastSeq})
		}
	}
}

func (client *governedQQWebSocket) write(value any) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	client.mu.Lock()
	conn := client.conn
	client.mu.Unlock()
	if conn == nil {
		return errors.New("qq: websocket is not connected")
	}
	return conn.WriteJSON(value)
}

func (client *governedQQWebSocket) Close() {
	client.mu.Lock()
	conn := client.conn
	client.conn = nil
	client.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
