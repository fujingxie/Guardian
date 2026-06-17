package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/guardian/agent/internal/conn"
	"github.com/guardian/agent/internal/enroll"
	"github.com/guardian/agent/internal/state"
)

const (
	agentVersion      = "0.1.0"
	heartbeatInterval = 15 * time.Second
)

func main() {
	var (
		enrollToken = flag.String("token", "", "一次性接入令牌（首次启动时必填）")
		consoleURL  = flag.String("console", "", "控制台 URL，例如 https://guardian.example.com")
		insecure    = flag.Bool("insecure", false, "跳过 TLS 校验（仅本地 localhost 自签时使用）")
	)
	flag.Parse()

	store := state.NewStore()
	st, err := store.Load()
	if err != nil {
		log.Fatalf("[agent] load state: %v", err)
	}

	if st == nil {
		if *enrollToken == "" || *consoleURL == "" {
			log.Fatalf("[agent] 未 enroll，必须传 --token 和 --console")
		}
		log.Printf("[agent] enrolling at %s", *consoleURL)
		resp, err := enroll.Do(*consoleURL, *enrollToken, *insecure)
		if err != nil {
			log.Fatalf("[agent] enroll: %v", err)
		}
		st = &state.State{
			ServerID:   resp.ServerID,
			AgentToken: resp.AgentToken,
			ConsoleURL: *consoleURL,
		}
		if err := store.Save(st); err != nil {
			log.Fatalf("[agent] save state: %v", err)
		}
		log.Printf("[agent] enrolled as %s", st.ServerID)
	} else {
		log.Printf("[agent] resumed as %s (console=%s)", st.ServerID, st.ConsoleURL)
		if *consoleURL != "" && *consoleURL != st.ConsoleURL {
			st.ConsoleURL = *consoleURL
			_ = store.Save(st)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loop := &conn.Loop{
		ConsoleURL: st.ConsoleURL,
		AgentToken: st.AgentToken,
		Insecure:   *insecure,
		OnConnect:  onConnect(ctx, st),
		OnMessage:  onMessage,
	}
	loop.Run(ctx)
}

func onConnect(ctx context.Context, st *state.State) func(send chan<- conn.Envelope) {
	return func(send chan<- conn.Envelope) {
		hn, _ := os.Hostname()
		hello, _ := json.Marshal(map[string]any{
			"serverId":     st.ServerID,
			"hostname":     hn,
			"os":           runtime.GOOS,
			"arch":         runtime.GOARCH,
			"agentVersion": agentVersion,
		})
		safeSend(send, conn.Envelope{Type: "hello", Payload: hello})

		// 每次成功连接，启动一个心跳 goroutine；连接断开后 send 阻塞 → 退出。
		go func() {
			t := time.NewTicker(heartbeatInterval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					payload, _ := json.Marshal(map[string]any{
						"ts": time.Now().UTC().Format(time.RFC3339),
					})
					if !safeSend(send, conn.Envelope{Type: "heartbeat", Payload: payload}) {
						return // 通道阻塞表明连接已死，等下一次 OnConnect 重新拉起
					}
				}
			}
		}()
	}
}

func onMessage(env conn.Envelope, _ chan<- conn.Envelope) {
	log.Printf("[agent] recv %s", env.Type)
}

// safeSend 非阻塞地往 send 投一个包；通道满则放弃（避免连接死掉的 goroutine 卡住）。
func safeSend(ch chan<- conn.Envelope, env conn.Envelope) bool {
	select {
	case ch <- env:
		return true
	case <-time.After(time.Second):
		return false
	}
}
