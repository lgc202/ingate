package clickhouse

import (
	"math"
	"testing"
	"time"

	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// TestTrafficMetricsRowRestoreDurations 验证合法延迟边界能够还原且保留流量计数。
func TestTrafficMetricsRowRestoreDurations(t *testing.T) {
	row := trafficMetricsRow{
		RequestCount:     12,
		ClientErrorCount: 2,
		ServerErrorCount: 1,
		NoResponseCount:  1,
		averageNanos:     uint64(4 * time.Millisecond),
		p50Nanos:         0,
		p95Nanos:         math.MaxInt64,
		p99Nanos:         math.MaxInt64,
	}
	if err := row.restoreDurations(); err != nil {
		t.Fatalf("restoreDurations(%+v) error = %v, want nil", row, err)
	}
	want := traffic.Metrics{
		RequestCount:     12,
		ClientErrorCount: 2,
		ServerErrorCount: 1,
		NoResponseCount:  1,
		AverageDuration:  4 * time.Millisecond,
		P50Duration:      0,
		P95Duration:      time.Duration(math.MaxInt64),
		P99Duration:      time.Duration(math.MaxInt64),
	}
	if row.Metrics != want {
		t.Errorf("restoreDurations() metrics = %+v, want %+v", row.Metrics, want)
	}
}

// TestTrafficMetricsRowRestoreDurationsPreservesRowOnError 验证任一延迟无效时整行保持原值。
func TestTrafficMetricsRowRestoreDurationsPreservesRowOnError(t *testing.T) {
	tests := []struct {
		name    string
		average uint64
		p50     uint64
		p95     uint64
		p99     uint64
	}{
		{name: "average_overflow", average: math.MaxInt64 + 1, p50: 1, p95: 2, p99: 3},
		{name: "p50_overflow", average: 1, p50: math.MaxInt64 + 1, p95: 2, p99: 3},
		{name: "p95_overflow", average: 1, p50: 1, p95: math.MaxInt64 + 1, p99: 3},
		{name: "p99_overflow", average: 1, p50: 1, p95: 2, p99: math.MaxInt64 + 1},
		{name: "p50_after_p95", average: 1, p50: 3, p95: 2, p99: 4},
		{name: "p95_after_p99", average: 1, p50: 1, p95: 3, p99: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := trafficMetricsRow{
				RequestCount:    12,
				AverageDuration: 10 * time.Millisecond,
				P50Duration:     11 * time.Millisecond,
				P95Duration:     12 * time.Millisecond,
				P99Duration:     13 * time.Millisecond,
				averageNanos:    test.average,
				p50Nanos:        test.p50,
				p95Nanos:        test.p95,
				p99Nanos:        test.p99,
			}
			before := row
			if err := row.restoreDurations(); err == nil {
				t.Errorf("restoreDurations(%+v) error = nil, want invalid duration error", before)
			}
			if row != before {
				t.Errorf("restoreDurations(%+v) changed row to %+v, want unchanged row", before, row)
			}
		})
	}
}
