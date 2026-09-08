// Package prometheus 组装进程级 Prometheus 指标端点。
package prometheus

import (
	"net/http"

	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler 创建仅包含进程基础指标和显式传入指标的独立端点。
// 独立 Registry 避免依赖库通过全局注册表意外暴露指标，也让各组件可以按需组合 Collector。
func NewHandler(componentCollectors ...prometheusclient.Collector) http.Handler {
	registry := prometheusclient.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	registry.MustRegister(componentCollectors...)

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}
