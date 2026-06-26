package hardening

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type Executor struct {
	mockDir string
}

func NewExecutor() *Executor {
	mockDir := ""
	if runtime.GOOS != "linux" {
		mockDir = "./mock_etc"
		log.Printf("[executor] running in MOCK mode (Darwin/macOS detected), redirected config to %s", mockDir)
	}
	return &Executor{mockDir: mockDir}
}

func isAllowedPath(path string) bool {
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, "/") {
		return false
	}
	allowedPrefixes := []string{
		"/etc/ssh/",
		"/etc/ufw/",
		"/etc/fail2ban/",
		"/etc/apt/",
	}
	for _, pref := range allowedPrefixes {
		if strings.HasPrefix(cleanPath, pref) {
			return true
		}
	}
	return false
}

func updateSSHConfigLine(content, option, value string) string {
	re := regexp.MustCompile("(?i)(?m)^[#\\s]*" + regexp.QuoteMeta(option) + "\\s+.*$")
	newLine := option + " " + value
	if re.MatchString(content) {
		return re.ReplaceAllString(content, newLine)
	}
	if !strings.HasSuffix(content, "\n") && content != "" {
		content += "\n"
	}
	return content + newLine + "\n"
}

func (e *Executor) getCurrentSSHPorts() []string {
	p := e.GetConfigPath("/etc/ssh/sshd_config")
	data, err := os.ReadFile(p)
	if err != nil {
		return []string{"22"}
	}
	var ports []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "port ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ports = append(ports, fields[1])
			}
		}
	}
	if len(ports) == 0 {
		return []string{"22"}
	}
	return ports
}

// GetConfigPath 映射文件真实物理路径（如果是 Mock，返回 ./mock_etc/...）
func (e *Executor) GetConfigPath(path string) string {
	if e.mockDir == "" {
		return path
	}
	// 将 /etc/ssh/sshd_config 映射为 ./mock_etc/etc/ssh/sshd_config
	clean := filepath.Clean(path)
	clean = strings.TrimPrefix(clean, "/")
	return filepath.Join(e.mockDir, clean)
}

// EnsureDefaultFile 保证配置文件存在，若不存在写入初始 mock 默认内容
func (e *Executor) EnsureDefaultFile(path, defaultContent string) error {
	p := e.GetConfigPath(path)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(defaultContent), 0o600)
}

// Execute 执行某项加固动作，返回修改前的文件快照 Map (filepath -> fileContent)
func (e *Executor) Execute(ctx context.Context, key string, adminIP string) (map[string]string, error) {
	snapshot := make(map[string]string)

	switch key {
	case "ssh_no_password":
		// 1. 预检 (仅在 Linux 执行真正的预检，避免锁死)
		if e.mockDir == "" {
			if err := e.precheckSSHKeys(); err != nil {
				return nil, fmt.Errorf("SSH 预检失败: %w", err)
			}
		}

		// 2. 准备配置文件默认内容
		defaultSSH := "PasswordAuthentication yes\nPermitRootLogin yes\nPort 22\nMaxAuthTries 6\n"
		if err := e.EnsureDefaultFile("/etc/ssh/sshd_config", defaultSSH); err != nil {
			return nil, err
		}

		// 3. 备份原始文件
		p := e.GetConfigPath("/etc/ssh/sshd_config")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ssh/sshd_config"] = string(oldData)

		// 4. 执行替换
		newContent := updateSSHConfigLine(string(oldData), "PasswordAuthentication", "no")
		if err := os.WriteFile(p, []byte(newContent), 0o600); err != nil {
			return nil, err
		}

		// 5. 重启 SSH
		if err := e.restartSSHService(); err != nil {
			return nil, err
		}

	case "ssh_port":
		defaultSSH := "PasswordAuthentication yes\nPermitRootLogin yes\nPort 22\nMaxAuthTries 6\n"
		if err := e.EnsureDefaultFile("/etc/ssh/sshd_config", defaultSSH); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/ssh/sshd_config")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ssh/sshd_config"] = string(oldData)

		// 默认改到 49222 端口
		newContent := updateSSHConfigLine(string(oldData), "Port", "49222")
		if err := os.WriteFile(p, []byte(newContent), 0o600); err != nil {
			return nil, err
		}

		if err := e.restartSSHService(); err != nil {
			return nil, err
		}

	case "ssh_no_root":
		defaultSSH := "PasswordAuthentication yes\nPermitRootLogin yes\nPort 22\nMaxAuthTries 6\n"
		if err := e.EnsureDefaultFile("/etc/ssh/sshd_config", defaultSSH); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/ssh/sshd_config")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ssh/sshd_config"] = string(oldData)

		newContent := updateSSHConfigLine(string(oldData), "PermitRootLogin", "no")
		if err := os.WriteFile(p, []byte(newContent), 0o600); err != nil {
			return nil, err
		}

		if err := e.restartSSHService(); err != nil {
			return nil, err
		}

	case "ufw":
		defaultUFW := "# Default user rules\n"
		if err := e.EnsureDefaultFile("/etc/ufw/user.rules", defaultUFW); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/ufw/user.rules")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ufw/user.rules"] = string(oldData)

		if e.mockDir == "" {
			// Linux 环境下执行 ufw
			if err := exec.Command("ufw", "default", "deny", "incoming").Run(); err != nil {
				return nil, fmt.Errorf("ufw default deny incoming: %w", err)
			}
			if err := exec.Command("ufw", "default", "allow", "outgoing").Run(); err != nil {
				return nil, fmt.Errorf("ufw default allow outgoing: %w", err)
			}
			
			// 动态读取并放行当前 SSH 实际运行的全部端口
			sshPorts := e.getCurrentSSHPorts()
			for _, port := range sshPorts {
				if err := exec.Command("ufw", "allow", port+"/tcp").Run(); err != nil {
					return nil, fmt.Errorf("ufw allow ssh port %s: %w", port, err)
				}
			}
			
			// 额外防范，放行默认与安全加固的 SSH 端口作为兜底
			for _, port := range []string{"22", "49222"} {
				found := false
				for _, p := range sshPorts {
					if p == port {
						found = true
						break
					}
				}
				if !found {
					if err := exec.Command("ufw", "allow", port+"/tcp").Run(); err != nil {
						return nil, fmt.Errorf("ufw allow fallback ssh port %s: %w", port, err)
					}
				}
			}

			if adminIP != "" {
				if err := exec.Command("ufw", "allow", "from", adminIP).Run(); err != nil {
					return nil, fmt.Errorf("ufw allow from adminIP %s: %w", adminIP, err)
				}
			}
			if err := exec.Command("ufw", "--force", "enable").Run(); err != nil {
				return nil, fmt.Errorf("enable ufw: %w", err)
			}
		} else {
			// Mock 模式
			newContent := string(oldData) + "\n# UFW ENABLED\n"
			sshPorts := e.getCurrentSSHPorts()
			for _, port := range sshPorts {
				newContent += fmt.Sprintf("# ALLOW Port %s\n", port)
			}
			newContent += "# ALLOW Port 22\n# ALLOW Port 49222\n"
			if adminIP != "" {
				newContent += fmt.Sprintf("# ALLOW from %s\n", adminIP)
			}
			_ = os.WriteFile(p, []byte(newContent), 0o600)
		}

	case "ufw_ports":
		defaultUFW := "# Default user rules\n"
		if err := e.EnsureDefaultFile("/etc/ufw/user.rules", defaultUFW); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/ufw/user.rules")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ufw/user.rules"] = string(oldData)

		if e.mockDir == "" {
			if err := exec.Command("ufw", "allow", "80/tcp").Run(); err != nil {
				return nil, fmt.Errorf("ufw allow port 80: %w", err)
			}
			if err := exec.Command("ufw", "allow", "443/tcp").Run(); err != nil {
				return nil, fmt.Errorf("ufw allow port 443: %w", err)
			}
		} else {
			newContent := string(oldData) + "\n# ALLOW Port 80\n# ALLOW Port 443\n"
			_ = os.WriteFile(p, []byte(newContent), 0o600)
		}

	case "fail2ban":
		defaultF2B := "[DEFAULT]\n"
		if err := e.EnsureDefaultFile("/etc/fail2ban/jail.local", defaultF2B); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/fail2ban/jail.local")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/fail2ban/jail.local"] = string(oldData)

		if e.mockDir == "" {
			// 安装 fail2ban
			if err := exec.Command("apt-get", "update").Run(); err != nil {
				return nil, fmt.Errorf("apt-get update: %w", err)
			}
			if err := exec.Command("apt-get", "install", "-y", "fail2ban").Run(); err != nil {
				return nil, fmt.Errorf("apt-get install fail2ban: %w", err)
			}

			// 写入配置
			f2bConfig := "[sshd]\nenabled = true\nport = ssh\nfilter = sshd\nmaxretry = 5\nbantime = 10m\n"
			if err := os.WriteFile("/etc/fail2ban/jail.local", []byte(f2bConfig), 0o644); err != nil {
				return nil, fmt.Errorf("write jail.local config error: %w", err)
			}
			if err := exec.Command("systemctl", "restart", "fail2ban").Run(); err != nil {
				return nil, fmt.Errorf("restart fail2ban service: %w", err)
			}
		} else {
			newContent := string(oldData) + "\n[sshd]\nenabled = true\n"
			_ = os.WriteFile(p, []byte(newContent), 0o600)
		}

	case "login_limit":
		defaultSSH := "PasswordAuthentication yes\nPermitRootLogin yes\nPort 22\nMaxAuthTries 6\n"
		if err := e.EnsureDefaultFile("/etc/ssh/sshd_config", defaultSSH); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/ssh/sshd_config")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/ssh/sshd_config"] = string(oldData)

		newContent := updateSSHConfigLine(string(oldData), "MaxAuthTries", "3")
		if err := os.WriteFile(p, []byte(newContent), 0o600); err != nil {
			return nil, err
		}

		if err := e.restartSSHService(); err != nil {
			return nil, err
		}

	case "auto_update":
		defaultUpgrades := "// Default unattended upgrades\n"
		if err := e.EnsureDefaultFile("/etc/apt/apt.conf.d/50unattended-upgrades", defaultUpgrades); err != nil {
			return nil, err
		}

		p := e.GetConfigPath("/etc/apt/apt.conf.d/50unattended-upgrades")
		oldData, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		snapshot["/etc/apt/apt.conf.d/50unattended-upgrades"] = string(oldData)

		if e.mockDir == "" {
			if err := exec.Command("apt-get", "install", "-y", "unattended-upgrades").Run(); err != nil {
				return nil, fmt.Errorf("install unattended-upgrades: %w", err)
			}
			autoUpgradeConf := "APT::Periodic::Update-Package-Lists \"1\";\nAPT::Periodic::Unattended-Upgrade \"1\";\n"
			if err := os.WriteFile("/etc/apt/apt.conf.d/20auto-upgrades", []byte(autoUpgradeConf), 0o644); err != nil {
				return nil, fmt.Errorf("write 20auto-upgrades config error: %w", err)
			}
		} else {
			newContent := string(oldData) + "\n// AUTO UPDATE ENABLED\n"
			_ = os.WriteFile(p, []byte(newContent), 0o600)
		}

	default:
		return nil, fmt.Errorf("unknown hardening key: %s", key)
	}

	return snapshot, nil
}

// Rollback 根据文件快照数据还原系统配置，并重启对应服务
func (e *Executor) Rollback(ctx context.Context, files map[string]string) error {
	for path, content := range files {
		if !isAllowedPath(path) {
			return fmt.Errorf("安全拒绝：不允许回滚非加固白名单的物理路径 %s", path)
		}
		p := e.GetConfigPath(path)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			return err
		}
		log.Printf("[executor] rolled back config file: %s", path)
	}

	// 还原后重启对应服务
	if err := e.restartSSHService(); err != nil {
		return err
	}

	if e.mockDir == "" {
		// 还原后在真实 Linux 下执行必要恢复
		if _, ok := files["/etc/ufw/user.rules"]; ok {
			// 如果快照有防火墙配置，可能需要重载或禁用
			if err := exec.Command("ufw", "disable").Run(); err != nil {
				log.Printf("[executor] rollback disable ufw error: %v", err)
			}
		}
	}

	return nil
}

func (e *Executor) precheckSSHKeys() error {
	// 真正的 Linux 预检：确保 ~/.ssh/authorized_keys 存在且不为空
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p := filepath.Join(home, ".ssh/authorized_keys")
	info, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("没有检测到 SSH 授权公钥 (%s)，禁止关闭密码登录以防锁死", p)
	}
	if info.Size() == 0 {
		return fmt.Errorf("授权公钥文件 (%s) 为空，禁止关闭密码登录", p)
	}
	return nil
}

func (e *Executor) restartSSHService() error {
	if e.mockDir != "" {
		return nil // Mock 模式下不重启真实服务
	}
	// 尝试用 systemctl 重启 ssh 或 sshd
	if err := exec.Command("systemctl", "restart", "sshd").Run(); err != nil {
		if err := exec.Command("systemctl", "restart", "ssh").Run(); err != nil {
			return fmt.Errorf("restart ssh service: %w", err)
		}
	}
	return nil
}
