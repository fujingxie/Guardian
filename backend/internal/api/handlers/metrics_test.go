package handlers

import (
	"testing"
	"time"

	"github.com/guardian/backend/internal/store"
)

func TestParseMetricRange(t *testing.T) {
	cases := map[string]time.Duration{
		"1h":  time.Hour,
		"6h":  6 * time.Hour,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"1w":  7 * 24 * time.Hour,
		"bad": 24 * time.Hour,
		"8d":  24 * time.Hour,
	}

	for input, want := range cases {
		if got := parseMetricRange(input); got != want {
			t.Fatalf("parseMetricRange(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestSampleMetricPointsKeepsBoundaries(t *testing.T) {
	points := make([]store.MetricPoint, 10)
	for i := range points {
		points[i].CPUPct = float64(i)
	}

	got := sampleMetricPoints(points, 4)
	if len(got) != 4 {
		t.Fatalf("sampled length = %d, want 4", len(got))
	}
	if got[0].CPUPct != 0 {
		t.Fatalf("first point = %.0f, want 0", got[0].CPUPct)
	}
	if got[len(got)-1].CPUPct != 9 {
		t.Fatalf("last point = %.0f, want 9", got[len(got)-1].CPUPct)
	}
}
