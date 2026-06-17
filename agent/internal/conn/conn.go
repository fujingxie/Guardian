// Package conn 负责 agent ↔ 控制台的长连接（WSS）：
//   - 出站连接（agent 主动，控制台不开任何入站端口）
//   - 指数退避重连
//   - 双向 JSON 信封（type + payload）
package conn

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Envelope 是双向消息的统一结构。
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Loop 维持一条 WSS 长连接：收到的入站消息扔进 in，要发出去的写到 out。
// 任何一边断开会触发 Reconnect 循环。ctx 取消时优雅退出。
type Loop struct {
	ConsoleURL string
	AgentToken string
	Insecure   bool

	OnConnect func(send chan<- Envelope) // 每次连上后调用一次
	OnMessage func(env Envelope, send chan<- Envelope)
}

func (l *Loop) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := l.runOnce(ctx); err != nil {
			log.Printf("[agent] ws disconnected: %v (retry in %s)", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (l *Loop) runOnce(ctx context.Context) error {
	u, err := url.Parse(strings.TrimRight(l.ConsoleURL, "/"))
	if err != nil {
		return fmt.Errorf("parse console url: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path += "/api/agent/ws"

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: l.Insecure},
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+l.AgentToken)

	ws, _, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer ws.Close()
	log.Printf("[agent] ws connected to %s", u.String())

	send := make(chan Envelope, 16)
	if l.OnConnect != nil {
		l.OnConnect(send)
	}

	// 写循环
	writerErr := make(chan error, 1)
	go func() {
		defer close(writerErr)
		for {
			select {
			case <-ctx.Done():
				_ = ws.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"),
					time.Now().Add(time.Second))
				return
			case env, ok := <-send:
				if !ok {
					return
				}
				if err := ws.WriteJSON(env); err != nil {
					writerErr <- err
					return
				}
			}
		}
	}()

	// 读循环
	ws.SetReadLimit(1 << 20)
	for {
		var env Envelope
		if err := ws.ReadJSON(&env); err != nil {
			select {
			case e := <-writerErr:
				if e != nil {
					return fmt.Errorf("write: %w", e)
				}
			default:
			}
			return fmt.Errorf("read: %w", err)
		}
		if l.OnMessage != nil {
			l.OnMessage(env, send)
		}
	}
}
