package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/internal/authz/service"
)

// Readiness 提供运维接口所需的组件就绪状态。
type Readiness interface {
	Ready() bool
}

// NewHTTPServer 创建健康检查和就绪检查服务。
func NewHTTPServer(
	config *conf.Server,
	readiness Readiness,
	authorization *service.AuthorizationService,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
	})
	server.HandleFunc("/readyz", func(response http.ResponseWriter, _ *http.Request) {
		if !readiness.Ready() {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	})
	server.Handle("/metrics", metricsHandler(readiness, authorization))
	return server
}

func metricsHandler(readiness Readiness, authorization *service.AuthorizationService) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "ready",
			Help:      "Whether Caller configuration and the Redis rate counter are ready.",
		}, func() float64 { return boolMetric(readiness.Ready()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "checks_total",
			Help:      "Authorization checks received from Envoy.",
		}, func() float64 { return float64(authorization.Counters().Checks) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "allowed_total",
			Help:      "Requests allowed by Caller authorization and request rate limits.",
		}, func() float64 { return float64(authorization.Counters().Allowed) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "denied_total",
			Help:      "Requests denied by Caller authorization.",
		}, func() float64 { return float64(authorization.Counters().Denied) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "rate_limited_total",
			Help:      "Requests rejected because a request rate limit was exhausted.",
		}, func() float64 { return float64(authorization.Counters().RateLimited) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "authz",
			Name:      "errors_total",
			Help:      "Authorization checks that failed because execution dependencies were unavailable.",
		}, func() float64 { return float64(authorization.Counters().Errors) }),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
