package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	"github.com/lgc202/ingate/internal/assistant/conf"
	systemservice "github.com/lgc202/ingate/internal/assistant/service/system"
)

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
	logger *slog.Logger,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Filter(requestIDFilter, responseNoStoreFilter, recoveryFilter(logger)),
		kratoshttp.ResponseEncoder(responseEncoder),
	)
	assistantv1.RegisterSystemServiceHTTPServer(server, systemAPI)
	server.HandleFunc("/healthz", live)
	server.HandleFunc("/readyz", readinessProbe(systemAPI))
	return server
}

func live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func readinessProbe(systemAPI *systemservice.Service) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		readiness, err := systemAPI.GetReadiness(request.Context(), &emptypb.Empty{})
		if err != nil {
			kratoshttp.DefaultErrorEncoder(response, request, err)
			return
		}
		statusCode := http.StatusOK
		if readiness.GetStatus() != assistantv1.ReadinessStatus_READINESS_STATUS_READY {
			statusCode = http.StatusServiceUnavailable
		}
		components := make([]componentResponse, 0, len(readiness.GetComponents()))
		for _, component := range readiness.GetComponents() {
			components = append(components, componentResponse{
				Name:   component.GetName(),
				Status: statusText(component.GetStatus()),
			})
		}
		writeJSON(response, statusCode, readinessResponse{
			Status:     statusText(readiness.GetStatus()),
			Components: components,
		})
	}
}

func statusText(status assistantv1.ReadinessStatus) string {
	if status == assistantv1.ReadinessStatus_READINESS_STATUS_READY {
		return "ready"
	}
	return "unavailable"
}

func writeJSON(response http.ResponseWriter, statusCode int, value any) {
	response.Header().Set("Content-Type", jsonContentType)
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(value)
}
