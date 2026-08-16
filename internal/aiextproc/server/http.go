package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiextproc/conf"
)

// NewHTTPServer 创建健康检查和就绪检查服务
func NewHTTPServer(config *conf.Server) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready)
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, map[string]string{"status": "ok"})
}

func ready(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, map[string]string{"status": "ready"})
}

func writeJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}
