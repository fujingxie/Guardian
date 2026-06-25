package explain

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	rds            *redis.Client
	claudeAPIKey   string
	deepseekAPIKey string
	httpClient     *http.Client
}

func New(rds *redis.Client, claudeAPIKey, deepseekAPIKey string) *Service {
	return &Service{
		rds:            rds,
		claudeAPIKey:   claudeAPIKey,
		deepseekAPIKey: deepseekAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Translate 人话翻译入口。
// eventType: bruteforce_blocked 等
// sourceIP: 攻击/登录源IP，可为空
// detailRaw: 原始详情JSON，可为空
func (s *Service) Translate(ctx context.Context, eventType, sourceIP string, detailRaw []byte) (string, error) {
	// 1. 尝试静态匹配
	if msg, ok := s.matchStatic(eventType, sourceIP, detailRaw); ok {
		return msg, nil
	}

	// 2. 动态解释：如果配置了 DeepSeek 或 Claude API Key，进行动态解释，否则使用静态/默认兜底
	// 拼接输入信息，用 MD5 生成 Redis 缓存键
	inputStr := fmt.Sprintf("%s:%s:%s", eventType, sourceIP, string(detailRaw))
	h := md5.Sum([]byte(inputStr))
	cacheKey := "explain:cache:" + hex.EncodeToString(h[:])

	// 查缓存
	if s.rds != nil {
		if val, err := s.rds.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			return val, nil
		}
	}

	explanation := ""
	if s.deepseekAPIKey != "" {
		var err error
		explanation, err = s.askDeepSeek(ctx, eventType, sourceIP, string(detailRaw))
		if err != nil {
			log.Printf("[explain] DeepSeek API error: %v, falling back to default", err)
			explanation = s.defaultFallback(eventType, sourceIP, detailRaw)
		} else {
			// 缓存，TTL 24小时
			if s.rds != nil && explanation != "" {
				_ = s.rds.Set(ctx, cacheKey, explanation, 24*time.Hour).Err()
			}
		}
	} else if s.claudeAPIKey != "" {
		var err error
		explanation, err = s.askClaude(ctx, eventType, sourceIP, string(detailRaw))
		if err != nil {
			log.Printf("[explain] Claude API error: %v, falling back to default", err)
			explanation = s.defaultFallback(eventType, sourceIP, detailRaw)
		} else {
			// 缓存，TTL 24小时
			if s.rds != nil && explanation != "" {
				_ = s.rds.Set(ctx, cacheKey, explanation, 24*time.Hour).Err()
			}
		}
	} else {
		explanation = s.defaultFallback(eventType, sourceIP, detailRaw)
	}

	return explanation, nil
}

func (s *Service) matchStatic(eventType, sourceIP string, detailRaw []byte) (string, bool) {
	switch eventType {
	case "bruteforce_blocked":
		jail := ""
		if len(detailRaw) > 0 {
			var d map[string]any
			if err := json.Unmarshal(detailRaw, &d); err == nil {
				if j, ok := d["jail"].(string); ok {
					jail = j
				}
			}
		}
		if jail == "sshd" || jail == "" {
			return fmt.Sprintf("服务器检测到来自 IP %s 的连续恶意登录尝试，已被 Fail2ban 自动封禁以防爆破。", sourceIP), true
		}
		return fmt.Sprintf("服务器检测到针对 %s 服务的恶意行为，已将来源 IP %s 自动封禁。", jail, sourceIP), true

	case "new_login":
		return fmt.Sprintf("检测到新的管理员登录，来自 IP %s。", sourceIP), true

	case "offline":
		// offline 时，sourceIP 可能是服务器名称或主机名
		return fmt.Sprintf("服务器离线！Guardian 失去与服务器 %s 的长连接心跳，请检查服务器网络或 Agent 状态。", sourceIP), true
	}

	return "", false
}

func (s *Service) defaultFallback(eventType, sourceIP string, detailRaw []byte) string {
	ipStr := "未知IP"
	if sourceIP != "" {
		ipStr = sourceIP
	}
	detailStr := ""
	if len(detailRaw) > 0 {
		detailStr = "，详情：" + string(detailRaw)
	}
	return fmt.Sprintf("系统检测到安全事件（类型：%s），来源：%s%s。", eventType, ipStr, detailStr)
}

type claudeRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	Messages  []claudeRequestMessage `json:"messages"`
}

type claudeResponseContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type claudeResponse struct {
	Content []claudeResponseContent `json:"content"`
}

func (s *Service) askClaude(ctx context.Context, eventType, sourceIP, detailStr string) (string, error) {
	prompt := fmt.Sprintf(
		"你是一个 Linux 系统安全专家。请用一句简短、容易理解的中文大白话（不超过30字）解释以下系统安全事件：类型是 %s，IP 是 %s，详情是 %s。请直接输出解释文案，不要有任何前缀、后缀、引号或额外解释。",
		eventType, sourceIP, detailStr,
	)

	reqPayload := claudeRequest{
		Model:     "claude-3-5-sonnet-20240620",
		MaxTokens: 100,
		Messages: []claudeRequestMessage{
			{Role: "user", Content: prompt},
		},
	}

	rawReq, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(rawReq))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", s.claudeAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status code %d: %s", resp.StatusCode, string(body))
	}

	var respPayload claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", err
	}

	if len(respPayload.Content) > 0 {
		return strings.TrimSpace(respPayload.Content[0].Text), nil
	}

	return "", fmt.Errorf("empty response content")
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekRequest struct {
	Model    string            `json:"model"`
	Messages []deepSeekMessage `json:"messages"`
}

type deepSeekChoice struct {
	Message deepSeekMessage `json:"message"`
}

type deepSeekResponse struct {
	Choices []deepSeekChoice `json:"choices"`
}

func (s *Service) askDeepSeek(ctx context.Context, eventType, sourceIP, detailStr string) (string, error) {
	prompt := fmt.Sprintf(
		"你是一个 Linux 系统安全专家。请用一句简短、容易理解的中文大白话（不超过30字）解释以下系统安全事件：类型是 %s，IP 是 %s，详情是 %s。请直接输出解释文案，不要有任何前缀、后缀、引号或额外解释。",
		eventType, sourceIP, detailStr,
	)

	reqPayload := deepSeekRequest{
		Model: "deepseek-chat",
		Messages: []deepSeekMessage{
			{Role: "user", Content: prompt},
		},
	}

	rawReq, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(rawReq))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+s.deepseekAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status code %d: %s", resp.StatusCode, string(body))
	}

	var respPayload deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", err
	}

	if len(respPayload.Choices) > 0 {
		return strings.TrimSpace(respPayload.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("empty choices content")
}
