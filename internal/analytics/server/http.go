package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/analytics/data/clickhouse"
)

type pinger interface {
	Ping(context.Context) error
}

// NewHTTPServer 创建健康检查、就绪检查和 Prometheus 指标服务。
func NewHTTPServer(
	config *conf.Server,
	consumer *RequestConsumer,
	clickHouse *clickhouse.Store,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(httpConfig.GetTimeout().AsDuration(), consumer, clickHouse))
	server.Handle("/metrics", metricsHandler(consumer.counters))
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	// healthz 只表示进程仍能处理 HTTP，不探测外部依赖
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

// ready 在同一个请求期限内确认 Kafka 和 ClickHouse 当前都可访问
func ready(timeout time.Duration, kafka pinger, clickHouse pinger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), timeout)
		defer cancel()
		if err := kafka.Ping(ctx); err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{
				"status":     "unavailable",
				"dependency": "kafka",
			})
			return
		}
		if err := clickHouse.Ping(ctx); err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{
				"status":     "unavailable",
				"dependency": "clickhouse",
			})
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	}
}

// metricsHandler 使用进程独立 Registry 暴露 Go 指标和请求记录处理计数
func metricsHandler(counters func() requestCounters) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "analytics",
			Name:      "records_received_total",
			Help:      "Request record messages consumed from Kafka.",
		}, func() float64 { return float64(counters().received) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "analytics",
			Name:      "records_stored_total",
			Help:      "Request records stored in ClickHouse.",
		}, func() float64 { return float64(counters().stored) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "analytics",
			Name:      "records_invalid_total",
			Help:      "Malformed request record messages discarded from Kafka.",
		}, func() float64 { return float64(counters().invalid) }),
	)

	// 使用独立 Registry，避免依赖库隐式注册与业务无关的全局指标
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
