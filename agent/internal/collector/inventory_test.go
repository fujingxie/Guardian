package collector

import (
	"context"
	"testing"
)

func TestInventoryCollectorCollectAndHashDeDuplication(t *testing.T) {
	ic := NewInventoryCollector()
	ctx := context.Background()

	// 1. 第一次 Collect，应正常返回 Mock 或真机数据
	inv1, err := ic.Collect(ctx)
	if err != nil {
		t.Fatalf("first Collect failed: %v", err)
	}
	if inv1 == nil {
		t.Fatalf("expected first Collect to return data, got nil")
	}

	if len(inv1.Ports) == 0 {
		t.Errorf("expected collected ports, got empty")
	}
	if len(inv1.Services) == 0 {
		t.Errorf("expected collected services, got empty")
	}
	if len(inv1.Packages) == 0 {
		t.Errorf("expected collected packages, got empty")
	}

	// 2. 第二次 Collect，在内容未发生变化时，因为哈希完全一样，应该返回 nil (过滤重复上报)
	inv2, err := ic.Collect(ctx)
	if err != nil {
		t.Fatalf("second Collect failed: %v", err)
	}
	if inv2 != nil {
		t.Errorf("expected second Collect to return nil (deduplicated), but got data: %v", inv2)
	}

	// 3. 测试哈希变更后是否能重新收集
	ic.prevHash = ""
	inv3, err := ic.Collect(ctx)
	if err != nil {
		t.Fatalf("third Collect failed: %v", err)
	}
	if inv3 == nil {
		t.Errorf("expected third Collect to return data after clearing prevHash, got nil")
	}
}

func TestParseRPMRow(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		expectedVer  string
	}{
		{"openssh-server-8.0p1-19.el8.x86_64", "openssh-server", "8.0p1"},
		{"curl-7.61.1-12.el8.i686", "curl", "7.61.1"},
		{"nginx-1.24.0-2.el9.noarch", "nginx", "1.24.0"},
	}

	for _, tc := range tests {
		name, ver := parseRPMRow(tc.input)
		if name != tc.expectedName {
			t.Errorf("for %s, expected name %s, got %s", tc.input, tc.expectedName, name)
		}
		if ver != tc.expectedVer {
			t.Errorf("for %s, expected version %s, got %s", tc.input, tc.expectedVer, ver)
		}
	}
}

func TestSplitIPAndPort(t *testing.T) {
	tests := []struct {
		input        string
		expectedIP   string
		expectedPort int
	}{
		{"0.0.0.0:22", "0.0.0.0", 22},
		{"[::]:22", "::", 22},
		{"127.0.0.1.3306", "127.0.0.1", 3306},
		{"[fe80::1]:8080", "fe80::1", 8080},
	}

	for _, tc := range tests {
		ip, port := splitIPAndPort(tc.input)
		if ip != tc.expectedIP {
			t.Errorf("for %s, expected IP %s, got %s", tc.input, tc.expectedIP, ip)
		}
		if port != tc.expectedPort {
			t.Errorf("for %s, expected port %d, got %d", tc.input, tc.expectedPort, port)
		}
	}
}
