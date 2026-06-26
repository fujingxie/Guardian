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
	"sync"
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
	loc := s.lookupIP(ctx, sourceIP)

	// 1. 尝试静态匹配
	if msg, ok := s.matchStatic(eventType, sourceIP, loc, detailRaw); ok {
		return msg, nil
	}

	ipText := sourceIP
	if loc != "" && loc != "未知位置" {
		ipText = fmt.Sprintf("%s（归属地：%s）", sourceIP, loc)
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
		explanation, err = s.askDeepSeek(ctx, eventType, ipText, string(detailRaw))
		if err != nil {
			log.Printf("[explain] DeepSeek API error: %v, falling back to default", err)
			explanation = s.defaultFallback(eventType, sourceIP, loc, detailRaw)
		} else {
			// 缓存，TTL 24小时
			if s.rds != nil && explanation != "" {
				_ = s.rds.Set(ctx, cacheKey, explanation, 24*time.Hour).Err()
			}
		}
	} else if s.claudeAPIKey != "" {
		var err error
		explanation, err = s.askClaude(ctx, eventType, ipText, string(detailRaw))
		if err != nil {
			log.Printf("[explain] Claude API error: %v, falling back to default", err)
			explanation = s.defaultFallback(eventType, sourceIP, loc, detailRaw)
		} else {
			// 缓存，TTL 24小时
			if s.rds != nil && explanation != "" {
				_ = s.rds.Set(ctx, cacheKey, explanation, 24*time.Hour).Err()
			}
		}
	} else {
		explanation = s.defaultFallback(eventType, sourceIP, loc, detailRaw)
	}

	return explanation, nil
}

func (s *Service) matchStatic(eventType, sourceIP, loc string, detailRaw []byte) (string, bool) {
	ipText := sourceIP
	if loc != "" && loc != "未知位置" {
		ipText = fmt.Sprintf("%s (%s)", sourceIP, loc)
	}

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
			return fmt.Sprintf("服务器检测到来自 IP %s 的连续恶意登录尝试，已被 Fail2ban 自动封禁以防爆破。", ipText), true
		}
		return fmt.Sprintf("服务器检测到针对 %s 服务的恶意行为，已将来源 IP %s 自动封禁。", jail, ipText), true

	case "new_login":
		return fmt.Sprintf("检测到新的管理员登录，来自 IP %s。", ipText), true

	case "offline":
		// offline 时，sourceIP 可能是服务器名称或主机名
		return fmt.Sprintf("服务器离线！Guardian 失去与服务器 %s 的长连接心跳，请检查服务器网络或 Agent 状态。", sourceIP), true
	}

	return "", false
}

func (s *Service) defaultFallback(eventType, sourceIP, loc string, detailRaw []byte) string {
	ipStr := "未知IP"
	if sourceIP != "" {
		ipStr = sourceIP
		if loc != "" && loc != "未知位置" {
			ipStr = fmt.Sprintf("%s (%s)", sourceIP, loc)
		}
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

var ipCache sync.Map // 一级内存缓存，避免高频请求相同 IP

// lookupIP 通过 ip-api.com 查询 IP 地理位置并进行二级缓存（Redis 30天 + 本地内存）。
func (s *Service) lookupIP(ctx context.Context, ip string) string {
	if ip == "" || strings.HasPrefix(ip, "127.") || ip == "localhost" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return ""
	}

	// 1. 内存缓存读取
	if val, ok := ipCache.Load(ip); ok {
		return val.(string)
	}

	// 2. Redis 缓存读取
	cacheKey := "ip:location:" + ip
	if s.rds != nil {
		if val, err := s.rds.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			ipCache.Store(ip, val)
			return val
		}
	}

	// 3. 在线 API 查归属地
	loc := ""
	url := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err == nil {
		resp, err := s.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var res struct {
					Status     string `json:"status"`
					Country    string `json:"country"`
					RegionName string `json:"regionName"`
					City       string `json:"city"`
				}
				if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Status == "success" {
					parts := []string{}
					if res.Country != "" {
						parts = append(parts, res.Country)
					}
					if res.RegionName != "" && res.RegionName != res.Country {
						parts = append(parts, res.RegionName)
					}
					if res.City != "" && res.City != res.RegionName {
						parts = append(parts, res.City)
					}
					loc = strings.Join(parts, " ")
				}
			}
		}
	}

	if loc == "" {
		loc = "未知位置"
	}

	// 4. 写回缓存
	ipCache.Store(ip, loc)
	if s.rds != nil {
		_ = s.rds.Set(ctx, cacheKey, loc, 30*24*time.Hour).Err()
	}

	return loc
}
