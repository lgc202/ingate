package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	systembiz "github.com/lgc202/ingate/internal/assistant/biz/system"
	"github.com/lgc202/ingate/internal/assistant/conf"
	systemservice "github.com/lgc202/ingate/internal/assistant/service/system"
)

type readinessHandler struct {
	usecase *systembiz.Usecase
}

type readinessResponse struct {
	Status     string              `json:"status"`
	Components []componentResponse `json:"components"`
}

type componentResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// NewHTTPServer 创建系统状态 API、存活探针与就绪探针。
func NewHTTPServer(
	config *conf.Server,
	systemAPI *systemservice.Service,
	readiness *systembiz.Usecase,
	logger *slog.Logger,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Filter(requestIDFilter(), responseNoStoreFilter(), recoveryFilter(logger)),
		kratoshttp.ResponseEncoder(responseEncoder),
	)
	assistantv1.RegisterSystemServiceHTTPServer(server, systemAPI)
	handler := readinessHandler{usecase: readiness}
	server.HandleFunc("/healthz", live)
	server.HandleFunc("/readyz", handler.ready)
	return server
}

func live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h readinessHandler) ready(response http.ResponseWriter, request *http.Request) {
	report := h.usecase.Check(request.Context())
	statusCode := http.StatusOK
	if report.Status != systembiz.StatusReady {
		statusCode = http.StatusServiceUnavailable
	}
	components := make([]componentResponse, 0, len(report.Components))
	for _, component := range report.Components {
		components = append(components, componentResponse{
			Name:   component.Name,
			Status: statusText(component.Status),
		})
	}
	writeJSON(response, statusCode, readinessResponse{
		Status:     statusText(report.Status),
		Components: components,
	})
}

func statusText(status systembiz.Status) string {
	if status == systembiz.StatusReady {
		return "ready"
	}
	return "unavailable"
}

func writeJSON(response http.ResponseWriter, statusCode int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(value)
}
