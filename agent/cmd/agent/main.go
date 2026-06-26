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

	"github.com/guardian/agent/internal/collector"
	"github.com/guardian/agent/internal/conn"
	"github.com/guardian/agent/internal/deadman"
	"github.com/guardian/agent/internal/enroll"
	"github.com/guardian/agent/internal/hardening"
	"github.com/guardian/agent/internal/state"
)

const (
	agentVersion      = "0.1.0"
	heartbeatInterval = 15 * time.Second
	collectInterval   = 10 * time.Second
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

	col := collector.New()
	execObj := hardening.NewExecutor()
	dm := deadman.New()

	// 启动本地死人开关守护
	dm.StartMonitor(ctx, execObj)

	loop := &conn.Loop{
		ConsoleURL: st.ConsoleURL,
		AgentToken: st.AgentToken,
		Insecure:   *insecure,
		OnConnect:  onConnect(ctx, st, col),
		OnMessage:  onMessage(ctx, dm, execObj),
	}
	loop.Run(ctx)
}

func onConnect(ctx context.Context, st *state.State, col *collector.Collector) func(send chan<- conn.Envelope) {
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

		// 心跳
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
						return
					}
				}
			}
		}()

		// 指标采集
		go func() {
			// 立刻上报一次，避免界面长时间空
			if s, err := col.Sample(ctx); err == nil {
				if payload, err := json.Marshal(s); err == nil {
					safeSend(send, conn.Envelope{Type: "metrics", Payload: payload})
				}
			}
			t := time.NewTicker(collectInterval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					s, err := col.Sample(ctx)
					if err != nil {
						continue
					}
					payload, err := json.Marshal(s)
					if err != nil {
						continue
					}
					if !safeSend(send, conn.Envelope{Type: "metrics", Payload: payload}) {
						return
					}
				}
			}
		}()

		// Fail2ban 日志监听
		go collector.StartFail2banWatcher(ctx, send)
	}
}

func onMessage(ctx context.Context, dm *deadman.Deadman, execObj *hardening.Executor) func(env conn.Envelope, send chan<- conn.Envelope) {
	return func(env conn.Envelope, send chan<- conn.Envelope) {
		if env.Type != "command" {
			log.Printf("[agent] recv unknown message type: %s", env.Type)
			return
		}

		var cmd struct {
			Cmd     string            `json:"cmd"`
			JobID   string            `json:"jobId"`
			Key     string            `json:"key"`
			AdminIP string            `json:"adminIp"`
			Files   map[string]string `json:"files"`
		}

		if err := json.Unmarshal(env.Payload, &cmd); err != nil {
			log.Printf("[agent] parse command error: %v", err)
			return
		}

		log.Printf("[agent] command received: %s (job: %s)", cmd.Cmd, cmd.JobID)

		switch cmd.Cmd {
		case "run_hardening":
			go func() {
				snapshot, err := execObj.Execute(ctx, cmd.Key, cmd.AdminIP)
				if err != nil {
					log.Printf("[agent] execute hardening %s failed: %v", cmd.Key, err)
					reply := conn.Envelope{
						Type: "job_result",
						Payload: marshalJobResult(cmd.JobID, cmd.Key, "failed", nil, err.Error()),
					}
					safeSend(send, reply)
					return
				}

				// 高风险项，写入本地 trial 状态
				isHighRisk := cmd.Key == "ssh_no_password" || cmd.Key == "ssh_port" || cmd.Key == "ssh_no_root"
				if isHighRisk {
					// 试运行 5 分钟，自动计算回滚时间
					rollbackAt := time.Now().Add(5 * time.Minute)
					job := deadman.TrialJob{
						JobID:      cmd.JobID,
						Key:        cmd.Key,
						RollbackAt: rollbackAt,
						Files:      snapshot,
					}
					if err := dm.SetTrial(job); err != nil {
						log.Printf("[agent] save trial job to disk: %v", err)
					} else {
						log.Printf("[AUDIT] [Action:ApplyHardening] [Key:%s] [JobID:%s] [Status:trial]", cmd.Key, cmd.JobID)
					}
				} else {
					log.Printf("[AUDIT] [Action:ApplyHardening] [Key:%s] [JobID:%s] [Status:applied]", cmd.Key, cmd.JobID)
				}

				reply := conn.Envelope{
					Type: "job_result",
					Payload: marshalJobResult(cmd.JobID, cmd.Key, "success", snapshot, ""),
				}
				safeSend(send, reply)
			}()

		case "confirm_hardening":
			dm.Clear()
			log.Printf("[AUDIT] [Action:ConfirmHardening] [JobID:%s]", cmd.JobID)

		case "rollback":
			go func() {
				log.Printf("[agent] command forced rollback for job: %s", cmd.JobID)
				if err := execObj.Rollback(ctx, cmd.Files); err != nil {
					log.Printf("[agent] forced rollback error: %v", err)
				} else {
					dm.Clear()
					log.Printf("[AUDIT] [Action:RollbackHardening] [JobID:%s]", cmd.JobID)
				}
			}()

		default:
			log.Printf("[agent] unknown cmd action: %s", cmd.Cmd)
		}
	}
}

func marshalJobResult(jobID, key, status string, snapshot map[string]string, errStr string) []byte {
	out, _ := json.Marshal(map[string]any{
		"jobId":    jobID,
		"key":      key,
		"status":   status,
		"snapshot": snapshot,
		"error":    errStr,
	})
	return out
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
