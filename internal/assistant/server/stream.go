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

	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

const (
	fallbackFailureCode        = "REQUEST_FAILED"
	failureResourceNotFound    = "RESOURCE_NOT_FOUND"
	failureResourceConflict    = "RESOURCE_CONFLICT"
	failureExecutionInProgress = "EXECUTION_ALREADY_RUNNING"
)

func registerStreamRoutes(
	server *kratoshttp.Server,
	conversations *conversationbiz.Service,
	stream *conf.Stream,
	logger *slog.Logger,
) {
	router := server.Route("/")
	router.POST("/assistant/v1/conversations/{id}/messages:stream", chatHandler(conversations, logger))
	router.GET("/assistant/v1/executions/{id}/events", eventHandler(conversations, stream, logger))
}

type chatRequest struct {
	Content string `json:"content"`
}

func chatHandler(conversations *conversationbiz.Service, logger *slog.Logger) kratoshttp.HandlerFunc {
	return func(ctx kratoshttp.Context) error {
		actorID, err := actorID(ctx.Request())
		if err != nil {
			return err
		}
		ctx.Request().Body = http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxMessageBytes)
		var request chatRequest
		if err := json.NewDecoder(ctx.Request().Body).Decode(&request); err != nil {
			return kratoserrors.BadRequest("INVALID_ARGUMENT", "invalid request body")
		}
		request.Content = strings.TrimSpace(request.Content)
		if request.Content == "" {
			return kratoserrors.BadRequest("INVALID_ARGUMENT", "message content is required")
		}

		stream, err := newSSEWriter(ctx.Response())
		if err != nil {
			return kratoserrors.InternalServer("STREAM_UNSUPPORTED", "streaming is unavailable").WithCause(err)
		}
		stream.start()
		execution, err := conversations.Chat(ctx, actorID, ctx.Vars().Get("id"), request.Content, stream.write)
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		logExecutionFailure(ctx, logger, ctx.Request(), actorID, execution, err)
		// HTTP 响应已经开始，失败状态只能作为 SSE 事件发送；内部错误不会暴露给浏览器。
		_ = stream.write(conversationbiz.StreamEvent{
			Type: "execution.failed",
			Data: streamFailureCode(execution, err),
		})
		return nil
	}
}

func eventHandler(
	conversations *conversationbiz.Service,
	config *conf.Stream,
	logger *slog.Logger,
) kratoshttp.HandlerFunc {
	return func(ctx kratoshttp.Context) error {
		actorID, err := actorID(ctx.Request())
		if err != nil {
			return err
		}
		executionID := ctx.Vars().Get("id")
		if _, err := conversations.GetExecution(ctx, actorID, executionID); err != nil {
			return streamRequestError(err)
		}
		stream, err := newSSEWriter(ctx.Response())
		if err != nil {
			return kratoserrors.InternalServer("STREAM_UNSUPPORTED", "streaming is unavailable").WithCause(err)
		}
		stream.start()
		lastID := ctx.Request().Header.Get("Last-Event-ID")
		for {
			events, err := conversations.ReadEvents(ctx, executionID, lastID, 100, config.GetReadBlock().AsDuration())
			if err != nil {
				logStreamFailure(ctx, logger, ctx.Request(), executionID, err)
				return nil
			}
			for _, event := range events {
				if err := stream.write(event); err != nil {
					return nil
				}
				lastID = event.ID
				if event.Type == "execution.completed" || event.Type == "execution.failed" {
					return nil
				}
			}
			if len(events) == 0 {
				execution, err := conversations.GetExecution(ctx, actorID, executionID)
				if err != nil {
					logStreamFailure(ctx, logger, ctx.Request(), executionID, err)
					return nil
				}
				if execution.State != conversationbiz.StateRunning {
					return nil
				}
			}
		}
	}
}

func logExecutionFailure(
	ctx context.Context,
	logger *slog.Logger,
	request *http.Request,
	actorID string,
	execution conversationbiz.Execution,
	err error,
) {
	if errors.Is(err, conversationbiz.ErrNotFound) ||
		errors.Is(err, conversationbiz.ErrVersionConflict) ||
		errors.Is(err, conversationbiz.ErrExecutionRunning) {
		return
	}
	attrs := []any{
		"actor", actorID,
		"conversation_id", execution.ConversationID,
		"execution_id", execution.ID,
		"failure_code", execution.FailureCode,
		"request_id", request.Header.Get(requestid.Header),
		"err", err,
	}
	if errors.Is(err, conversationbiz.ErrModelNotConfigured) {
		logger.WarnContext(ctx, "assistant model is not configured", attrs...)
		return
	}
	logger.ErrorContext(ctx, "assistant execution failed", attrs...)
}

func streamFailureCode(execution conversationbiz.Execution, err error) string {
	if execution.FailureCode != "" {
		return execution.FailureCode
	}
	switch {
	case errors.Is(err, conversationbiz.ErrNotFound):
		return failureResourceNotFound
	case errors.Is(err, conversationbiz.ErrExecutionRunning):
		return failureExecutionInProgress
	case errors.Is(err, conversationbiz.ErrVersionConflict):
		return failureResourceConflict
	default:
		return fallbackFailureCode
	}
}

func streamRequestError(err error) error {
	switch {
	case errors.Is(err, conversationbiz.ErrNotFound):
		return kratoserrors.NotFound(failureResourceNotFound, "resource not found")
	case errors.Is(err, conversationbiz.ErrVersionConflict), errors.Is(err, conversationbiz.ErrExecutionRunning):
		return kratoserrors.Conflict(failureResourceConflict, "resource state changed")
	default:
		return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func logStreamFailure(
	ctx context.Context,
	logger *slog.Logger,
	request *http.Request,
	executionID string,
	err error,
) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	logger.ErrorContext(ctx, "read assistant event stream failed",
		"execution_id", executionID,
		"request_id", request.Header.Get(requestid.Header),
		"err", err,
	)
}

func actorID(request *http.Request) (string, error) {
	value := strings.TrimSpace(request.Header.Get(forwardedUserHeader))
	if value == "" {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
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

func (w *sseWriter) write(event conversationbiz.StreamEvent) error {
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
