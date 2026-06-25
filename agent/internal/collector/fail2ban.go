package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/guardian/agent/internal/conn"
)

var fail2banRegex = regexp.MustCompile(`(?i)NOTICE\s+\[([a-zA-Z0-9_-]+)\]\s+Ban\s+([a-zA-Z0-9.:]+)`)

func StartFail2banWatcher(ctx context.Context, send chan<- conn.Envelope) {
	logPath := "/var/log/fail2ban.log"
	if runtime.GOOS != "linux" {
		// macOS 或者开发调试环境
		logPath = "./mock_fail2ban.log"
	}

	// 保证目录存在
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[fail2ban] failed to mkdir for log: %v", err)
	}

	var file *os.File
	var offset int64
	var err error

	openFile := func() bool {
		file, err = os.Open(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				// 如果文件不存在，我们就以写入模式创建它（这对 Mock 特别有用）
				f, createErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
				if createErr == nil {
					f.Close()
					file, err = os.Open(logPath)
				}
			}
		}

		if err != nil {
			log.Printf("[fail2ban] failed to open log file %s: %v, will retry in 5s", logPath, err)
			return false
		}

		// 首次打开，我们需要定位到文件末尾，避免重新处理历史日志
		info, statErr := file.Stat()
		if statErr != nil {
			log.Printf("[fail2ban] stat error: %v", statErr)
			file.Close()
			return false
		}
		offset = info.Size()
		return true
	}

	// 循环尝试打开
	for {
		if openFile() {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
	defer file.Close()

	log.Printf("[fail2ban] watching log file: %s (initial offset: %d)", logPath, offset)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, statErr := os.Stat(logPath)
			if statErr != nil {
				// 文件可能被删了或在进行 logrotate，尝试重新打开
				log.Printf("[fail2ban] stat err (logrotate?): %v, reopening", statErr)
				file.Close()
				for {
					if openFile() {
						break
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
					}
				}
				continue
			}

			currSize := info.Size()
			if currSize == offset {
				continue
			}

			if currSize < offset {
				// 文件变小了，说明被截断或者轮转了，重置 offset 为 0
				log.Printf("[fail2ban] log file truncated/rotated. Resetting offset to 0")
				offset = 0
			}

			// 读取新增的数据
			_, seekErr := file.Seek(offset, io.SeekStart)
			if seekErr != nil {
				log.Printf("[fail2ban] seek error: %v", seekErr)
				continue
			}

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				match := fail2banRegex.FindStringSubmatch(line)
				if len(match) == 3 {
					jail := match[1]
					ip := match[2]

					log.Printf("[fail2ban] Ban alert captured. jail: %s, ip: %s", jail, ip)

					payload, marshalErr := json.Marshal(map[string]any{
						"ts":        time.Now().UTC().Format(time.RFC3339),
						"eventType": "bruteforce_blocked",
						"sourceIp":  ip,
						"detail": map[string]any{
							"jail": jail,
						},
					})
					if marshalErr == nil {
						// 写入 channel 发送
						select {
						case send <- conn.Envelope{Type: "event", Payload: payload}:
						case <-time.After(1 * time.Second):
							log.Printf("[fail2ban] send to socket chan timeout, dropped event")
						}
					}
				}
			}

			// 更新 offset
			offset = currSize
		}
	}
}
