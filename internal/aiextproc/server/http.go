package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/aiextproc/conf"
	"github.com/lgc202/ingate/internal/aiextproc/service"
)

// Readiness 提供运维接口所需的组件就绪状态
type Readiness interface {
	Ready() bool
}

// NewHTTPServer 创建健康检查和就绪检查服务
func NewHTTPServer(
	config *conf.Server,
	readiness Readiness,
	processor *service.ExternalProcessor,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", func(response http.ResponseWriter, _ *http.Request) {
		if !readiness.Ready() {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	})
	server.Handle("/metrics", metricsHandler(readiness, processor))
	return server
}

func metricsHandler(readiness Readiness, processor *service.ExternalProcessor) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "ai_extproc",
			Name:      "ready",
			Help:      "Whether AI route configuration and the Redis token counter are ready.",
		}, func() float64 { return boolMetric(readiness.Ready()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "ai_extproc",
			Name:      "streams_total",
			Help:      "External Processing streams received from Envoy.",
		}, func() float64 { return float64(processor.Counters().Streams) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "ai_extproc",
			Name:      "stream_errors_total",
			Help:      "External Processing streams terminated by protocol or execution errors.",
		}, func() float64 { return float64(processor.Counters().Errors) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "ai_extproc",
			Name:      "active_correlations",
			Help:      "Downstream AI requests currently waiting for upstream stream correlation.",
		}, func() float64 { return float64(processor.Counters().ActiveCorrelations) }),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
