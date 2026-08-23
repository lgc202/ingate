package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
)

// NewHTTPServer 创建 Controller 的健康检查与就绪检查接口
func NewHTTPServer(
	config *conf.Server,
	configDelivery *delivery.Delivery,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(configDelivery))
	server.Handle("/metrics", metricsHandler(configDelivery))
	return server
}

func metricsHandler(configDelivery *delivery.Delivery) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "controller",
			Name:      "ready",
			Help:      "Whether the Envoy configuration delivery loop is running.",
		}, func() float64 { return boolMetric(configDelivery.Ready()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "controller",
			Name:      "active_resources",
			Help:      "Declarative resource generations in the active Envoy configuration.",
		}, func() float64 { return float64(len(configDelivery.Status().ActiveResources)) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "controller",
			Name:      "active_policy_targets",
			Help:      "Policy target attachments in the active Envoy configuration.",
		}, func() float64 { return float64(len(configDelivery.Status().ActivePolicyTargets)) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "controller",
			Name:      "last_delivery_failed",
			Help:      "Whether the latest recorded Envoy configuration delivery result is a failure.",
		}, func() float64 { return boolMetric(configDelivery.Status().LastFailure != nil) }),
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

func ready(configDelivery *delivery.Delivery) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		if !configDelivery.Ready() {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
