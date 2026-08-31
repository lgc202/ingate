package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"

	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/service/identity"
	"github.com/lgc202/ingate/internal/pkg/adminidentity"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

const (
	failureResourceNotFound = "RESOURCE_NOT_FOUND"
	streamReadLimit         = 100
	maxStreamEventIDBytes   = 64
)

// StreamHandler 负责执行事件的 SSE 路由、连接生命周期和边界日志。
type StreamHandler struct {
	executions *executionbiz.Usecase
	config     *conf.Stream
	logger     *slog.Logger
}

type sseWriter struct {
	response http.ResponseWriter
	flusher  http.Flusher
}

// NewStreamHandler 创建 Assistant 事件流处理器，供进程装配层注入 HTTP Server。
func NewStreamHandler(
	executions *executionbiz.Usecase,
	config *conf.Stream,
	logger *slog.Logger,
) *StreamHandler {
	return &StreamHandler{executions: executions, config: config, logger: logger}
}

func newSSEWriter(response http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		return nil, errors.New("response writer does not support flushing")
	}
	return &sseWriter{response: response, flusher: flusher}, nil
}

func (h *StreamHandler) register(server *kratoshttp.Server) {
	router := server.Route("/")
	router.GET("/assistant/v1/executions/{id}/events", h.events)
}

func (h *StreamHandler) events(ctx kratoshttp.Context) error {
	actorID, err := identity.ValidateActorID(ctx.Request().Header.Get(adminidentity.Header))
	if err != nil {
		return err
	}
	executionID := ctx.Vars().Get("id")
	if uuid.Validate(executionID) != nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "execution ID is invalid")
	}
	if _, err := h.executions.Get(ctx, actorID, executionID); err != nil {
		return streamRequestError(err)
	}
	lastEventID := ctx.Request().Header.Get("Last-Event-ID")
	if !validStreamEventID(lastEventID) {
		return kerrors.BadRequest("INVALID_ARGUMENT", "Last-Event-ID is invalid")
	}
	stream, err := newSSEWriter(ctx.Response())
	if err != nil {
		return kerrors.InternalServer("STREAM_UNSUPPORTED", "streaming is unavailable").WithCause(err)
	}
	stream.start()
	for {
		events, err := h.executions.ReadEvents(
			ctx,
			executionID,
			lastEventID,
			streamReadLimit,
			h.config.GetReadBlock().AsDuration(),
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
			lastEventID = event.ID
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

func streamRequestError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return kerrors.ClientClosed("REQUEST_CANCELLED", "request cancelled").WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return kerrors.GatewayTimeout("REQUEST_TIMEOUT", "request timed out").WithCause(err)
	case errors.Is(err, executionbiz.ErrNotFound):
		return kerrors.NotFound(failureResourceNotFound, "resource not found")
	default:
		return kerrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func validStreamEventID(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maxStreamEventIDBytes {
		return false
	}
	millisecondsText, sequenceText, ok := strings.Cut(value, "-")
	if !ok || strings.Contains(sequenceText, "-") {
		return false
	}
	milliseconds, err := strconv.ParseUint(millisecondsText, 10, 64)
	if err != nil || strconv.FormatUint(milliseconds, 10) != millisecondsText {
		return false
	}
	sequence, err := strconv.ParseUint(sequenceText, 10, 64)
	return err == nil && strconv.FormatUint(sequence, 10) == sequenceText
}

func terminalEvent(item executionbiz.Execution) (executionbiz.StreamEvent, bool) {
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

func (h *StreamHandler) logReadFailure(
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

func (w *sseWriter) start() {
	w.response.Header().Set("Content-Type", "text/event-stream")
	w.response.Header().Set("Cache-Control", "no-cache, no-store")
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
