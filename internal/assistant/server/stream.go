package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

const (
	failureResourceNotFound = "RESOURCE_NOT_FOUND"
	maxActorIDLength        = 128
)

// streamHandler 负责自定义 SSE 路由、连接生命周期和边界日志。
type streamHandler struct {
	runs   *runbiz.Service
	config *conf.Stream
	logger *slog.Logger
}

func newStreamHandler(
	runs *runbiz.Service,
	config *conf.Stream,
	logger *slog.Logger,
) *streamHandler {
	return &streamHandler{runs: runs, config: config, logger: logger}
}

func (h *streamHandler) register(server *kratoshttp.Server) {
	router := server.Route("/")
	router.GET("/assistant/v1/runs/{id}/events", h.events)
}

func (h *streamHandler) events(ctx kratoshttp.Context) error {
	actorID, err := h.actorID(ctx.Request())
	if err != nil {
		return err
	}
	runID := ctx.Vars().Get("id")
	if _, err := h.runs.Get(ctx, actorID, runID); err != nil {
		return h.requestError(err)
	}
	stream, err := newSSEWriter(ctx.Response())
	if err != nil {
		return kratoserrors.InternalServer("STREAM_UNSUPPORTED", "streaming is unavailable").WithCause(err)
	}
	stream.start()
	lastID := ctx.Request().Header.Get("Last-Event-ID")
	for {
		events, err := h.runs.ReadEvents(
			ctx, runID, lastID, 100, h.config.GetReadBlock().AsDuration(),
		)
		if err != nil {
			h.logReadFailure(ctx, ctx.Request(), runID, err)
			_ = stream.write(runbiz.StreamEvent{
				Type: runbiz.EventStreamFailed,
				Data: "EVENT_STREAM_UNAVAILABLE",
			})
			return nil
		}
		for _, event := range events {
			if err := stream.write(event); err != nil {
				return nil
			}
			lastID = event.ID
			if event.Type == runbiz.EventCompleted ||
				event.Type == runbiz.EventFailed ||
				event.Type == runbiz.EventCancelled {
				return nil
			}
		}
		if len(events) == 0 {
			item, err := h.runs.Get(ctx, actorID, runID)
			if err != nil {
				h.logReadFailure(ctx, ctx.Request(), runID, err)
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

func (h *streamHandler) requestError(err error) error {
	switch {
	case errors.Is(err, runbiz.ErrNotFound):
		return kratoserrors.NotFound(failureResourceNotFound, "resource not found")
	default:
		return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func terminalEvent(item runbiz.Run) (runbiz.StreamEvent, bool) {
	switch item.State {
	case runbiz.StateSucceeded:
		return runbiz.StreamEvent{Type: runbiz.EventCompleted}, true
	case runbiz.StateFailed:
		return runbiz.StreamEvent{Type: runbiz.EventFailed, Data: string(item.ErrorCode)}, true
	case runbiz.StateCancelled:
		return runbiz.StreamEvent{Type: runbiz.EventCancelled}, true
	default:
		return runbiz.StreamEvent{}, false
	}
}

func (h *streamHandler) logReadFailure(
	ctx context.Context,
	request *http.Request,
	runID string,
	err error,
) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	h.logger.ErrorContext(ctx, "read assistant event stream failed",
		"run_id", runID,
		"request_id", request.Header.Get(requestid.Header),
		"err", err,
	)
}

func (h *streamHandler) actorID(request *http.Request) (string, error) {
	value := strings.TrimSpace(request.Header.Get(forwardedUserHeader))
	if value == "" {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
	}
	if len(value) > maxActorIDLength {
		return "", kratoserrors.BadRequest("INVALID_ARGUMENT", "actor identifier is too long")
	}
	return value, nil
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

func (w *sseWriter) write(event runbiz.StreamEvent) error {
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
