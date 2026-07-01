package notify

import "testing"

func TestNormalizeAlertType(t *testing.T) {
	cases := map[string]string{
		"bruteforce_blocked": "bruteforce",
		"bruteforce":         "bruteforce",
		"port_scan":          "port_scan",
		"new_login":          "new_login",
		"cpu_usage_high":     "metric_threshold",
		"mem_usage_high":     "metric_threshold",
		"disk_usage_high":    "metric_threshold",
		"offline":            "offline",
		"something_new":      "unknown",
	}

	for input, want := range cases {
		if got := NormalizeAlertType(input); got != want {
			t.Fatalf("NormalizeAlertType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAlertTypeEnabledDefaultsToAllow(t *testing.T) {
	if !alertTypeEnabled(ChannelConfig{}, "bruteforce_blocked") {
		t.Fatal("missing alertTypes config should allow existing notifications")
	}

	cc := ChannelConfig{
		AlertTypes: map[string]bool{
			"bruteforce":       false,
			"metric_threshold": true,
		},
	}

	if alertTypeEnabled(cc, "bruteforce_blocked") {
		t.Fatal("bruteforce notifications should be disabled")
	}
	if !alertTypeEnabled(cc, "cpu_usage_high") {
		t.Fatal("metric threshold notifications should be enabled")
	}
	if !alertTypeEnabled(cc, "port_scan") {
		t.Fatal("missing alert type key should default to enabled")
	}
}
