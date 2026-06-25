package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
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
	Email      string `json:"email"`
	Telegram   string `json:"telegram"`
	ServerChan string `json:"serverChan"`
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

// Send 核心推送逻辑。读取当前配置，向所有启用的通道发送通知。
func (s *Service) Send(ctx context.Context, title, message string) {
	ns, err := s.alertsStore.GetNotifySettings(ctx)
	if err != nil {
		log.Printf("[notify] failed to get notify settings: %v", err)
		return
	}

	if !ns.Enabled {
		log.Printf("[notify] global notification is disabled")
		return
	}

	var cc ChannelConfig
	if len(ns.Channels) > 0 {
		if err := json.Unmarshal(ns.Channels, &cc); err != nil {
			log.Printf("[notify] failed to unmarshal channels: %v", err)
			return
		}
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

	auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
	
	// 组装标准的 MIME 邮件，并指定 Content-Type 保证中文不乱码
	fromName := "Guardian Admin"
	if s.smtpCfg.From != "" {
		fromName = s.smtpCfg.From
	}

	msg := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", fromName, s.smtpCfg.User, to, subject, body)

	addr := fmt.Sprintf("%s:%s", s.smtpCfg.Host, s.smtpCfg.Port)
	return smtp.SendMail(addr, auth, s.smtpCfg.User, []string{to}, []byte(msg))
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
