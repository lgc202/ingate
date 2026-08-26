package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/service/identity"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

const failureResourceNotFound = "RESOURCE_NOT_FOUND"

// executionStreamHandler 负责执行事件的 SSE 路由、连接生命周期和边界日志。
type executionStreamHandler struct {
	executions *executionbiz.Service
	config     *conf.Stream
	logger     *slog.Logger
}

// NewStreamHandler 创建 Assistant 事件流处理器，供进程装配层注入 HTTP Server。
func NewStreamHandler(
	executions *executionbiz.Service,
	config *conf.Stream,
	logger *slog.Logger,
) *executionStreamHandler {
	return &executionStreamHandler{executions: executions, config: config, logger: logger}
}

func (h *executionStreamHandler) register(server *kratoshttp.Server) {
	router := server.Route("/")
	router.GET("/assistant/v1/executions/{id}/events", h.events)
}

func (h *executionStreamHandler) events(ctx kratoshttp.Context) error {
	actorID, err := identity.ValidateActorID(ctx.Request().Header.Get(identity.ForwardedUserHeader))
	if err != nil {
		return err
	}
	executionID := ctx.Vars().Get("id")
	if _, err := h.executions.Get(ctx, actorID, executionID); err != nil {
		return h.requestError(err)
	}
	stream, err := newSSEWriter(ctx.Response())
	if err != nil {
		return kratoserrors.InternalServer("STREAM_UNSUPPORTED", "streaming is unavailable").WithCause(err)
	}
	stream.start()
	lastID := ctx.Request().Header.Get("Last-Event-ID")
	for {
		events, err := h.executions.ReadEvents(
			ctx, executionID, lastID, 100, h.config.GetReadBlock().AsDuration(),
		)
		if err != nil {
			h.logReadFailure(ctx, ctx.Request(), executionID, err)
			_ = stream.write(executionbiz.StreamEvent{
				Type: executionbiz.EventStreamFailed,
				Data: "EVENT_STREAM_UNAVAILABLE",
			})
			return nil
		}
		for _, event := range events {
			if err := stream.write(event); err != nil {
				return nil
			}
			lastID = event.ID
			if event.Type == executionbiz.EventCompleted ||
				event.Type == executionbiz.EventFailed ||
				event.Type == executionbiz.EventCancelled {
				return nil
			}
		}
		if len(events) == 0 {
			item, err := h.executions.Get(ctx, actorID, executionID)
			if err != nil {
				h.logReadFailure(ctx, ctx.Request(), executionID, err)
				return nil
			}
			if event, terminal := terminalEvent(item); terminal {
				_ = stream.write(event)
				return nil
			}
			if err := stream.heartbeat(); err != nil {
				return nil
			}
		}
	}
}

func (h *executionStreamHandler) requestError(err error) error {
	switch {
	case errors.Is(err, executionbiz.ErrNotFound):
		return kratoserrors.NotFound(failureResourceNotFound, "resource not found")
	default:
		return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func terminalEvent(item executionbiz.AgentExecution) (executionbiz.StreamEvent, bool) {
	switch item.State {
	case executionbiz.StateSucceeded:
		return executionbiz.StreamEvent{Type: executionbiz.EventCompleted}, true
	case executionbiz.StateFailed:
		return executionbiz.StreamEvent{Type: executionbiz.EventFailed, Data: string(item.ErrorCode)}, true
	case executionbiz.StateCancelled:
		return executionbiz.StreamEvent{Type: executionbiz.EventCancelled}, true
	default:
		return executionbiz.StreamEvent{}, false
	}
}

func (h *executionStreamHandler) logReadFailure(
	ctx context.Context,
	request *http.Request,
	executionID string,
	err error,
) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	h.logger.ErrorContext(ctx, "read assistant event stream failed",
		"execution_id", executionID,
		"request_id", request.Header.Get(requestid.Header),
		"err", err,
	)
}

type sseWriter struct {
	response http.ResponseWriter
	flusher  http.Flusher
}

func newSSEWriter(response http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}
	return &sseWriter{response: response, flusher: flusher}, nil
}

func (w *sseWriter) start() {
	w.response.Header().Set("Content-Type", "text/event-stream")
	w.response.Header().Set("Cache-Control", "no-cache")
	w.response.Header().Set("X-Accel-Buffering", "no")
	w.response.WriteHeader(http.StatusOK)
	w.flusher.Flush()
}

func (w *sseWriter) write(event executionbiz.StreamEvent) error {
	data, err := json.Marshal(map[string]string{"value": event.Data})
	if err != nil {
		return fmt.Errorf("encode SSE event: %w", err)
	}
	if event.ID != "" {
		if _, err := fmt.Fprintf(w.response, "id: %s\n", event.ID); err != nil {
			return fmt.Errorf("write SSE event ID: %w", err)
		}
	}
	if _, err := fmt.Fprintf(w.response, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
		return fmt.Errorf("write SSE event: %w", err)
	}
	w.flusher.Flush()
	return nil
}

func (w *sseWriter) heartbeat() error {
	if _, err := fmt.Fprint(w.response, ": heartbeat\n\n"); err != nil {
		return fmt.Errorf("write SSE heartbeat: %w", err)
	}
	w.flusher.Flush()
	return nil
}
