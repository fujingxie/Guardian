package collector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type PortItem struct {
	Port    int    `json:"port"`
	Proto   string `json:"proto"`
	Addr    string `json:"addr"`
	Pid     int    `json:"pid"`
	Process string `json:"process"`
}

type ServiceItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      string `json:"active"`
}

type PackageItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InventoryData struct {
	TS       time.Time     `json:"ts"`
	Ports    []PortItem    `json:"ports"`
	Services []ServiceItem `json:"services"`
	Packages []PackageItem `json:"packages"`
}

type InventoryCollector struct {
	prevHash string
}

func NewInventoryCollector() *InventoryCollector {
	return &InventoryCollector{}
}

// Collect 抓取系统画像，若内容无变化则返回 nil, nil
func (ic *InventoryCollector) Collect(ctx context.Context) (*InventoryData, error) {
	ports, err := collectPorts(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect ports failed: %w", err)
	}

	services, err := collectServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect services failed: %w", err)
	}

	packages, err := collectPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect packages failed: %w", err)
	}

	// 计算当前状态数据的哈希用于差分去重 (排除随机时间戳 ts)
	rawStruct := struct {
		Ports    []PortItem    `json:"ports"`
		Services []ServiceItem `json:"services"`
		Packages []PackageItem `json:"packages"`
	}{
		Ports:    ports,
		Services: services,
		Packages: packages,
	}

	bytesData, err := json.Marshal(rawStruct)
	if err != nil {
		return nil, fmt.Errorf("marshal inventory failed: %w", err)
	}

	sum := sha256.Sum256(bytesData)
	currentHash := hex.EncodeToString(sum[:])

	if currentHash == ic.prevHash {
		// 无变化，返回空
		return nil, nil
	}

	ic.prevHash = currentHash

	return &InventoryData{
		TS:       time.Now().UTC(),
		Ports:    ports,
		Services: services,
		Packages: packages,
	}, nil
}

func collectPorts(ctx context.Context) ([]PortItem, error) {
	if runtime.GOOS != "linux" {
		return getMockPorts(), nil
	}

	// 优先使用 ss
	out, err := runCmd(ctx, "ss", "-tlnp")
	if err == nil {
		return parseSSOutput(out), nil
	}

	// fallback 使用 netstat
	out, err = runCmd(ctx, "netstat", "-tlnp")
	if err == nil {
		return parseNetstatOutput(out), nil
	}

	return nil, fmt.Errorf("both ss and netstat failed: %w", err)
}

func parseSSOutput(output string) []PortItem {
	var items []PortItem
	scanner := bufio.NewScanner(strings.NewReader(output))
	ssUsersRx := regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "State") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		localAddr := fields[3]
		ip, portVal := splitIPAndPort(localAddr)
		if portVal == 0 {
			continue
		}

		proto := "tcp"
		var pid int
		var process string

		// 最后一列或中间某列包含 users:
		usersCol := ""
		for _, f := range fields {
			if strings.Contains(f, "users:") {
				usersCol = f
				break
			}
		}
		if usersCol != "" {
			matches := ssUsersRx.FindStringSubmatch(usersCol)
			if len(matches) >= 3 {
				process = matches[1]
				pid, _ = strconv.Atoi(matches[2])
			}
		}

		items = append(items, PortItem{
			Port:    portVal,
			Proto:   proto,
			Addr:    ip,
			Pid:     pid,
			Process: process,
		})
	}
	return items
}

func parseNetstatOutput(output string) []PortItem {
	var items []PortItem
	scanner := bufio.NewScanner(strings.NewReader(output))
	netstatUsersRx := regexp.MustCompile(`(\d+)/([^\s]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Active") || strings.HasPrefix(line, "Proto") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		proto := fields[0]
		if strings.HasPrefix(proto, "tcp") {
			proto = "tcp"
		}

		localAddr := fields[3]
		ip, portVal := splitIPAndPort(localAddr)
		if portVal == 0 {
			continue
		}

		var pid int
		var process string
		lastCol := fields[len(fields)-1]
		matches := netstatUsersRx.FindStringSubmatch(lastCol)
		if len(matches) >= 3 {
			pid, _ = strconv.Atoi(matches[1])
			process = matches[2]
		}

		items = append(items, PortItem{
			Port:    portVal,
			Proto:   proto,
			Addr:    ip,
			Pid:     pid,
			Process: process,
		})
	}
	return items
}

func splitIPAndPort(addr string) (string, int) {
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		idx = strings.LastIndex(addr, ".")
		if idx == -1 {
			return addr, 0
		}
	}
	ip := addr[:idx]
	portStr := addr[idx+1:]
	portVal, err := strconv.Atoi(portStr)
	if err != nil {
		return addr, 0
	}
	ip = strings.TrimPrefix(ip, "[")
	ip = strings.TrimSuffix(ip, "]")
	if ip == "" || ip == "::" {
		ip = "::"
	}
	return ip, portVal
}

func collectServices(ctx context.Context) ([]ServiceItem, error) {
	if runtime.GOOS != "linux" {
		return getMockServices(), nil
	}

	out, err := runCmd(ctx, "systemctl", "list-units", "--type=service", "--state=running", "--no-legend", "--no-pager")
	if err != nil {
		return nil, err
	}

	var items []ServiceItem
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		} else {
			desc = name
		}
		active := fields[3]

		items = append(items, ServiceItem{
			Name:        name,
			Description: desc,
			Active:      active,
		})
	}
	return items, nil
}

func collectPackages(ctx context.Context) ([]PackageItem, error) {
	if runtime.GOOS != "linux" {
		return getMockPackages(), nil
	}

	if isDebian() {
		out, err := runCmd(ctx, "dpkg", "-l")
		if err != nil {
			return nil, err
		}
		var items []PackageItem
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "ii") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					items = append(items, PackageItem{
						Name:    fields[1],
						Version: fields[2],
					})
				}
			}
		}
		return items, nil
	}

	if isRHEL() {
		out, err := runCmd(ctx, "rpm", "-qa")
		if err != nil {
			return nil, err
		}
		var items []PackageItem
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			name, version := parseRPMRow(line)
			items = append(items, PackageItem{
				Name:    name,
				Version: version,
			})
		}
		return items, nil
	}

	return nil, fmt.Errorf("neither dpkg nor rpm package manager found on this Linux system")
}

func isDebian() bool {
	_, err := exec.LookPath("dpkg")
	return err == nil
}

func isRHEL() bool {
	_, err := exec.LookPath("rpm")
	return err == nil
}

func parseRPMRow(row string) (string, string) {
	idxArch := strings.LastIndex(row, ".")
	if idxArch != -1 {
		row = row[:idxArch]
	}
	idxRelease := strings.LastIndex(row, "-")
	if idxRelease == -1 {
		return row, "unknown"
	}
	row = row[:idxRelease]
	idxVersion := strings.LastIndex(row, "-")
	if idxVersion == -1 {
		return row, "unknown"
	}
	name := row[:idxVersion]
	version := row[idxVersion+1:]
	return name, version
}

func runCmd(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("cmd %s %v failed: %w, stderr: %s", name, args, err, stderr.String())
	}
	return stdout.String(), nil
}

func getMockPorts() []PortItem {
	return []PortItem{
		{Port: 22, Proto: "tcp", Addr: "0.0.0.0", Pid: 123, Process: "sshd"},
		{Port: 80, Proto: "tcp", Addr: "0.0.0.0", Pid: 124, Process: "nginx"},
		{Port: 3306, Proto: "tcp", Addr: "127.0.0.1", Pid: 125, Process: "mysqld"},
		{Port: 8080, Proto: "tcp", Addr: "::", Pid: 126, Process: "node"},
	}
}

func getMockServices() []ServiceItem {
	return []ServiceItem{
		{Name: "ssh.service", Description: "OpenBSD Secure Shell server", Active: "running"},
		{Name: "nginx.service", Description: "A high performance web server and a reverse proxy server", Active: "running"},
		{Name: "mysql.service", Description: "MySQL Community Server", Active: "running"},
		{Name: "ufw.service", Description: "CLI frontend for managing a runtime firewall", Active: "running"},
		{Name: "fail2ban.service", Description: "Ban hosts that cause multiple authentication errors", Active: "running"},
	}
}

func getMockPackages() []PackageItem {
	packages := []PackageItem{
		{Name: "openssh-server", Version: "1:8.9p1-3ubuntu1"},
		{Name: "nginx", Version: "1.24.0-2ubuntu7"},
		{Name: "mysql-server", Version: "8.0.35-0ubuntu0.22.04.1"},
		{Name: "ufw", Version: "0.36.1-4build1"},
		{Name: "fail2ban", Version: "0.11.2-2"},
	}
	// 生成 1000+ 个应用包，以对 react-window 虚拟化进行极致测试
	for i := 1; i <= 1000; i++ {
		packages = append(packages, PackageItem{
			Name:    fmt.Sprintf("mock-package-%d", i),
			Version: fmt.Sprintf("1.0.%d-ubuntu1", i),
		})
	}
	return packages
}
