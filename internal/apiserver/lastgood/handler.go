package lastgood

import (
	"encoding/json"
	"errors"
	"net/http"

	envoylastgood "github.com/lgc202/ingate/internal/envoy/lastgood"
)

type handler struct {
	store *Store
}

// NewHandler 创建 Envoy Last Good 内部单例接口
func NewHandler(store *Store) http.Handler {
	return &handler{store: store}
}

func (h *handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		h.get(response, request)
	case http.MethodPut:
		h.put(response, request)
	default:
		response.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handler) get(response http.ResponseWriter, request *http.Request) {
	record, err := h.store.load(request.Context())
	if errors.Is(err, envoylastgood.ErrNotFound) {
		http.Error(response, "last good not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(response, "failed to load last good", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(record)
	if err != nil {
		http.Error(response, "failed to encode last good", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(data) // HTTP server 负责处理客户端提前断开
}

func (h *handler) put(response http.ResponseWriter, request *http.Request) {
	record, err := envoylastgood.Decode(request.Body)
	if err != nil {
		http.Error(response, "invalid last good record", http.StatusBadRequest)
		return
	}
	if err := h.store.save(request.Context(), record); err != nil {
		http.Error(response, "failed to save last good", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
