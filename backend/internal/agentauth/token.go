package agentauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewEnrollmentToken 生成一次性接入令牌（明文给用户复制；服务端只存哈希）。
func NewEnrollmentToken() string {
	return newToken("enroll_")
}

// NewAgentToken 生成长期 agent 令牌；agent enroll 成功后替换 enrollment token。
func NewAgentToken() string {
	return newToken("agent_")
}

// Hash 对令牌做 SHA-256，用于落库与比对。注意：不是为了对抗 GPU 爆破（agent token 自带 64hex 熵足够），
// 而是为了避免直接落明文带来的额外泄露面。
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken(prefix string) string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand 失败属于系统级别灾难，让上层 panic 表面化
		panic(err)
	}
	return prefix + hex.EncodeToString(buf[:])
}
