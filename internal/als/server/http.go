package server

import (
	"context"
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/kafka"
)

// NewHTTPServer 创建健康检查、就绪检查和 Prometheus 指标服务
func NewHTTPServer(config *conf.Bootstrap, writer *kafka.Writer, recorder *biz.Recorder) *kratoshttp.Server {
	httpConfig := config.GetServer().GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(config.GetData(), writer, recorder))
	server.Handle("/metrics", newMetricsHandler(recorder, config.GetData().GetDiskQueue().GetMaxBytes()))
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"status": "ok"})
}

func ready(config *conf.Data, writer *kafka.Writer, recorder *biz.Recorder) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), config.GetKafka().GetReadinessTimeout().AsDuration())
		defer cancel()
		status := recorder.DeliveryStatus()
		kafkaErr := writer.Ping(ctx)
		queueFull := status.PendingBytes >= config.GetDiskQueue().GetMaxBytes()
		canQueue := status.QueueWritable && !queueFull
		canWriteKafka := kafkaErr == nil && !status.Spooling
		if !canWriteKafka && !canQueue {
			writeJSON(response, http.StatusServiceUnavailable, map[string]any{
				"status":          "unavailable",
				"delivery":        "none",
				"pending_records": status.PendingRecords,
				"pending_bytes":   status.PendingBytes,
			})
			return
		}
		delivery := "kafka"
		if !canWriteKafka {
			// Kafka 短暂故障不应立即摘除 ALS；只要 WAL 仍可写，组件就能继续无损接收记录
			delivery = "disk_queue"
		}
		writeJSON(response, http.StatusOK, map[string]any{
			"status":          "ready",
			"delivery":        delivery,
			"pending_records": status.PendingRecords,
			"pending_bytes":   status.PendingBytes,
		})
	}
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
