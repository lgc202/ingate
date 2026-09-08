package prometheus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerIncludesRuntimeAndProcessMetrics 验证独立 Registry 保留基础运行指标。
func TestHandlerIncludesRuntimeAndProcessMetrics(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, name := range []string{"go_gc_duration_seconds", "process_cpu_seconds_total"} {
		if !strings.Contains(body, name) {
			t.Errorf("GET /metrics response does not contain %q", name)
		}
	}
}
