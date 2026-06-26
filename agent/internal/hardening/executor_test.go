package hardening

import (
	"strings"
	"testing"
)

func TestIsAllowedPath(t *testing.T) {
	tests := []struct {
		path    string
		allowed bool
	}{
		{"/etc/ssh/sshd_config", true},
		{"/etc/ufw/user.rules", true},
		{"/etc/fail2ban/jail.local", true},
		{"/etc/apt/apt.conf.d/50unattended-upgrades", true},
		{"/etc/cron.d/malicious", false},
		{"/etc/ssh/../../etc/cron.d/malicious", false}, // 穿越测试
		{"/root/.ssh/authorized_keys", false},
		{"relative/path/to/config", false},
	}

	for _, tt := range tests {
		res := isAllowedPath(tt.path)
		if res != tt.allowed {
			t.Errorf("path %q: expected allowed=%v, got %v", tt.path, tt.allowed, res)
		}
	}
}

func TestUpdateSSHConfigLine(t *testing.T) {
	// 场景 1：原有选项开启且值不同，执行替换
	config1 := "Port 22\nPasswordAuthentication yes\n"
	res1 := updateSSHConfigLine(config1, "PasswordAuthentication", "no")
	if !strings.Contains(res1, "PasswordAuthentication no") || strings.Contains(res1, "PasswordAuthentication yes") {
		t.Errorf("failed to update PasswordAuthentication, got: %q", res1)
	}

	// 场景 2：原有选项被注释，执行取消注释并更新值
	config2 := "#Port 22\n# PermitRootLogin yes\n"
	res2 := updateSSHConfigLine(config2, "PermitRootLogin", "no")
	if !strings.Contains(res2, "PermitRootLogin no") || strings.Contains(res2, "# PermitRootLogin") {
		t.Errorf("failed to uncomment and update PermitRootLogin, got: %q", res2)
	}

	// 场景 3：原有选项完全不存在，执行追加
	config3 := "Port 22\n"
	res3 := updateSSHConfigLine(config3, "MaxAuthTries", "3")
	if !strings.Contains(res3, "MaxAuthTries 3") {
		t.Errorf("failed to append MaxAuthTries, got: %q", res3)
	}

	// 场景 4：选项名大小写不敏感匹配
	config4 := "port 22\n"
	res4 := updateSSHConfigLine(config4, "Port", "49222")
	if !strings.Contains(res4, "Port 49222") || strings.Contains(res4, "port 22") {
		t.Errorf("failed to replace option with different case, got: %q", res4)
	}
}
