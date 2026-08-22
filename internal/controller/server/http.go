package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
)

// NewHTTPServer 创建 Controller 的运维接口，并向同机 Envoy 提供已校验的 Wasm 模块
func NewHTTPServer(
	config *conf.Server,
	configDelivery *delivery.Delivery,
	wasmModules http.Handler,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(configDelivery))
	server.HandlePrefix("/internal/wasm/", wasmModules)
	return server
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
