package agentauth

import (
	"strings"
	"testing"
)

func TestNewTokens(t *testing.T) {
	enroll := NewEnrollmentToken()
	if !strings.HasPrefix(enroll, "enroll_") {
		t.Errorf("expected enroll token prefix, got %s", enroll)
	}

	agent := NewAgentToken()
	if !strings.HasPrefix(agent, "agent_") {
		t.Errorf("expected agent token prefix, got %s", agent)
	}

	// 验证长度（前缀 + 64位十六进制）
	if len(enroll) != 7+64 {
		t.Errorf("unexpected enroll token length: %d", len(enroll))
	}
}

func TestTokenHash(t *testing.T) {
	tok1 := "my-secret-token"
	tok2 := "my-secret-token"
	tok3 := "different-token"

	h1 := Hash(tok1)
	h2 := Hash(tok2)
	h3 := Hash(tok3)

	if h1 != h2 {
		t.Errorf("expected same hash for same input")
	}
	if h1 == h3 {
		t.Errorf("expected different hash for different input")
	}

	// 确认哈希长度是 64 个字符（SHA-256 十六进制）
	if len(h1) != 64 {
		t.Errorf("unexpected hash length: %d", len(h1))
	}
}
