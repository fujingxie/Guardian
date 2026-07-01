package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/guardian/backend/internal/store"
)

type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

type ChannelConfig struct {
	Email      string          `json:"email"`
	Telegram   string          `json:"telegram"`
	ServerChan string          `json:"serverChan"`
	AlertTypes map[string]bool `json:"alertTypes"`
	Enabled    struct {
		Email      bool `json:"email"`
		Telegram   bool `json:"telegram"`
		ServerChan bool `json:"serverChan"`
	} `json:"enabled"`
}

type Service struct {
	alertsStore *store.Alerts
	smtpCfg     SMTPConfig
	httpClient  *http.Client

	cacheMu   sync.RWMutex
	cachedNS  *store.NotifySettings
	cacheTime time.Time
}

func New(alertsStore *store.Alerts, smtpCfg SMTPConfig) *Service {
	return &Service{
		alertsStore: alertsStore,
		smtpCfg:     smtpCfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) InvalidateCache() {
	s.cacheMu.Lock()
	s.cachedNS = nil
	s.cacheMu.Unlock()
}

// GetSettings 获取通知配置（包含 1 分钟缓存逻辑）
func (s *Service) GetSettings(ctx context.Context) (*store.NotifySettings, error) {
	s.cacheMu.RLock()
	if s.cachedNS != nil && time.Since(s.cacheTime) < time.Minute {
		ns := s.cachedNS
		s.cacheMu.RUnlock()
		return ns, nil
	}
	s.cacheMu.RUnlock()

	ns, err := s.alertsStore.GetNotifySettings(ctx)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cachedNS = ns
	s.cacheTime = time.Now()
	s.cacheMu.Unlock()

	return ns, nil
}

// NormalizeAlertType maps low-level event names to the user-facing alert type keys.
func NormalizeAlertType(eventType string) string {
	switch eventType {
	case "":
		return ""
	case "bruteforce", "bruteforce_blocked":
		return "bruteforce"
	case "port_scan":
		return "port_scan"
	case "new_login":
		return "new_login"
	case "cpu_usage_high", "mem_usage_high", "disk_usage_high", "metric_threshold":
		return "metric_threshold"
	case "offline":
		return "offline"
	default:
		return "unknown"
	}
}

func alertTypeEnabled(cc ChannelConfig, eventType string) bool {
	alertType := NormalizeAlertType(eventType)
	if alertType == "" || cc.AlertTypes == nil {
		return true
	}
	enabled, ok := cc.AlertTypes[alertType]
	if !ok {
		return true
	}
	return enabled
}

// Send 核心推送逻辑。读取当前配置，向所有启用的通道发送通知。
func (s *Service) Send(ctx context.Context, title, message string) {
	s.SendAlert(ctx, "", title, message)
}

// SendAlert 与 Send 一样发送通知，但会先检查该告警类型是否允许推送。
func (s *Service) SendAlert(ctx context.Context, eventType, title, message string) {
	ns, err := s.GetSettings(ctx)
	if err != nil {
		log.Printf("[notify] failed to get notify settings: %v", err)
		return
	}

	if !ns.Enabled {
		return
	}

	var cc ChannelConfig
	if len(ns.Channels) > 0 {
		if err := json.Unmarshal(ns.Channels, &cc); err != nil {
			log.Printf("[notify] failed to unmarshal channels: %v", err)
			return
		}
	}
	if !alertTypeEnabled(cc, eventType) {
		log.Printf("[notify] skipped alert notification by type filter: %s", NormalizeAlertType(eventType))
		return
	}

	// 1. Email 推送
	if cc.Enabled.Email && cc.Email != "" {
		go func() {
			if err := s.sendEmail(cc.Email, title, message); err != nil {
				log.Printf("[notify] email failed: %v", err)
			} else {
				log.Printf("[notify] email sent to %s", cc.Email)
			}
		}()
	}

	// 2. Telegram 推送
	if cc.Enabled.Telegram && cc.Telegram != "" {
		go func() {
			if err := s.sendTelegram(cc.Telegram, title+"\n\n"+message); err != nil {
				log.Printf("[notify] telegram failed: %v", err)
			} else {
				log.Printf("[notify] telegram notification sent")
			}
		}()
	}

	// 3. Server酱 推送
	if cc.Enabled.ServerChan && cc.ServerChan != "" {
		go func() {
			if err := s.sendServerChan(cc.ServerChan, title, message); err != nil {
				log.Printf("[notify] serverChan failed: %v", err)
			} else {
				log.Printf("[notify] serverChan notification sent")
			}
		}()
	}
}

// TestChannel 测试单一通道。
func (s *Service) TestChannel(ctx context.Context, channel string) error {
	ns, err := s.alertsStore.GetNotifySettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get notify settings: %w", err)
	}

	var cc ChannelConfig
	if len(ns.Channels) > 0 {
		if err := json.Unmarshal(ns.Channels, &cc); err != nil {
			return fmt.Errorf("failed to unmarshal channels: %w", err)
		}
	}

	testTitle := "Guardian 通道测试"
	testMsg := fmt.Sprintf("这是一条来自 Guardian 服务器安全管家的测试通知。发送时间: %s。这表明您的通道配置正确且可用。", time.Now().Local().Format("2006-01-02 15:04:05"))

	switch channel {
	case "email":
		if cc.Email == "" {
			return errors.New("未配置测试接收邮箱")
		}
		return s.sendEmail(cc.Email, testTitle, testMsg)

	case "telegram":
		if cc.Telegram == "" {
			return errors.New("未配置 Telegram 接收信息")
		}
		return s.sendTelegram(cc.Telegram, testTitle+"\n\n"+testMsg)

	case "serverChan":
		if cc.ServerChan == "" {
			return errors.New("未配置 Server酱 SCKEY")
		}
		return s.sendServerChan(cc.ServerChan, testTitle, testMsg)

	default:
		return fmt.Errorf("unknown channel: %s", channel)
	}
}

func (s *Service) sendEmail(to, subject, body string) error {
	if s.smtpCfg.Host == "" {
		return errors.New("SMTP 服务器未配置（环境变量 SMTP_HOST 为空）")
	}

	// 过滤换行以防止邮件头注入
	replacer := strings.NewReplacer("\r", "", "\n", "")
	to = replacer.Replace(to)
	subject = replacer.Replace(subject)

	fromName := "Guardian Admin"
	if s.smtpCfg.From != "" {
		fromName = replacer.Replace(s.smtpCfg.From)
	}

	msg := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", fromName, s.smtpCfg.User, to, subject, body)

	addr := fmt.Sprintf("%s:%s", s.smtpCfg.Host, s.smtpCfg.Port)
	auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)

	// 1. 如果端口是 465，通常是 SSL/TLS (隐式 TLS)
	if s.smtpCfg.Port == "465" {
		tlsConfig := &tls.Config{
			ServerName: s.smtpCfg.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial error: %w", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, s.smtpCfg.Host)
		if err != nil {
			return fmt.Errorf("smtp client error: %w", err)
		}
		defer c.Close()

		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth error: %w", err)
		}

		if err = c.Mail(s.smtpCfg.User); err != nil {
			return err
		}
		if err = c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(msg))
		if err != nil {
			return err
		}
		_ = w.Close()
		return c.Quit()
	}

	// 2. 否则使用普通 TCP 拨号，并探测 STARTTLS
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial error: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: s.smtpCfg.Host}
		if err = c.StartTLS(config); err != nil {
			return fmt.Errorf("smtp starttls error: %w", err)
		}
	}

	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth error: %w", err)
	}
	if err = c.Mail(s.smtpCfg.User); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	_ = w.Close()
	return c.Quit()
}

func (s *Service) sendTelegram(configStr, text string) error {
	// configStr 格式一般为 "chat_id|bot_token" 或 "chat_id"（如果没有 bot_token 可能会有其它设计，但在这里要求是 chat_id|bot_token）
	parts := strings.Split(configStr, "|")
	if len(parts) != 2 {
		return errors.New("Telegram 格式有误，正确格式为: chat_id|bot_token")
	}
	chatID := parts[0]
	botToken := parts[1]

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned status: %s", resp.Status)
	}
	return nil
}

func (s *Service) sendServerChan(sckey, title, desp string) error {
	apiURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", sckey)

	formData := url.Values{}
	formData.Set("title", title)
	formData.Set("desp", desp)

	resp, err := s.httpClient.PostForm(apiURL, formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ServerChan API returned status: %s", resp.Status)
	}
	return nil
}
