package ginshow

import "testing"

func TestPercentile(t *testing.T) {
	samples := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(samples, 95); got != 10 {
		t.Fatalf("expected p95=10, got %v", got)
	}
	if got := percentile(samples, 99); got != 10 {
		t.Fatalf("expected p99=10, got %v", got)
	}
}

func TestLatencyRingPercentiles(t *testing.T) {
	var ring latencyRing
	for _, ms := range []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 100} {
		ring.add(ms)
	}

	p95, p99 := ring.percentiles()
	if p95 < 9 || p95 > 100 {
		t.Fatalf("unexpected p95=%v", p95)
	}
	if p99 < 9 || p99 > 100 {
		t.Fatalf("unexpected p99=%v", p99)
	}
}
