package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string        // 监听地址，例如 :8080
	AccessToken    string        // 控制台访问口令（环境变量唯一来源）
	DatabaseURL    string        // postgres://user:pass@host:port/dbname?sslmode=disable
	RedisURL       string        // redis://localhost:6379/0
	ClaudeAPIKey   string        // 可选：人话兜底
	TrialDuration  time.Duration // 死人开关默认倒计时（min 解析）
	ConsoleBaseURL string        // 拼安装命令用，例如 https://guardian.example.com
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
	DeepSeekAPIKey string
}

func MustLoad() Config {
	c := Config{
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		AccessToken:    env("ACCESS_TOKEN", ""),
		DatabaseURL:    env("DATABASE_URL", "postgres://guardian:guardian@localhost:5432/guardian?sslmode=disable"),
		RedisURL:       env("REDIS_URL", "redis://localhost:6379/0"),
		ClaudeAPIKey:   env("ANTHROPIC_API_KEY", ""),
		ConsoleBaseURL: env("CONSOLE_BASE_URL", "https://localhost"),
		SMTPHost:       env("SMTP_HOST", ""),
		SMTPPort:       env("SMTP_PORT", "587"),
		SMTPUser:       env("SMTP_USER", ""),
		SMTPPass:       env("SMTP_PASS", ""),
		SMTPFrom:       env("SMTP_FROM", ""),
		DeepSeekAPIKey: env("DEEPSEEK_API_KEY", ""),
	}

	mins := env("TRIAL_MINUTES", "5")
	d, err := time.ParseDuration(mins + "m")
	if err != nil {
		panic(fmt.Errorf("invalid TRIAL_MINUTES=%q: %w", mins, err))
	}
	c.TrialDuration = d

	if strings.TrimSpace(c.AccessToken) == "" {
		// 不强行 panic：方便本地开发；启动日志会高亮警告。
		c.AccessToken = "guardian-dev-token"
	}
	return c
}

func env(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
