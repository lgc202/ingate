package wasm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/policy"
	quotaRedis "github.com/lgc202/ingate/plugins/tokenquota/internal/redis"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/usage"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	contentTypeHeader = "content-type"
	retryAfterHeader  = "retry-after"
	jsonContentType   = "application/json"
	sseContentType    = "text/event-stream"
)

// OnHttpRequestHeaders 在请求进入 AI Proxy 前检查当前固定窗口额度
func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok || len(route.config.Policies) == 0 {
		return types.ActionContinue
	}

	h.checks = policy.BuildChecks(route.config, requestFromProxyWasm(route))
	if len(h.checks) == 0 {
		return types.ActionContinue
	}
	h.checkOutcomes = make([]policy.CheckOutcome, len(h.checks))
	h.nextCheck = 0
	if h.dispatchNextCheck() {
		return types.ActionPause
	}

	decision := policy.Decide(h.checks, h.checkOutcomes)
	if !decision.Allowed {
		h.clearChecks()
		sendDecision(decision)
		return types.ActionPause
	}
	h.bookings = h.checks
	h.clearChecks()
	return types.ActionContinue
}

// OnHttpResponseHeaders 只对成功的 AI 响应提取并记账
func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	if len(h.bookings) == 0 {
		return types.ActionContinue
	}
	status, err := strconv.Atoi(pluginwasm.ResponseHeader(":status"))
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			proxywasm.LogErrorf("parse token quota response status failed: %v", err)
		}
		h.clearResponse()
		return types.ActionContinue
	}

	h.responseActive = true
	contentType := strings.ToLower(pluginwasm.ResponseHeader(contentTypeHeader))
	h.responseStreaming = strings.HasPrefix(contentType, sseContentType)
	if endOfStream {
		proxywasm.LogError("token quota response ended without usage")
		h.clearResponse()
	}
	return types.ActionContinue
}

// OnHttpResponseBody 在 AI Proxy 归一化后读取普通响应或 SSE 的最终 usage
func (h *httpContext) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {
	if !h.responseActive || len(h.bookings) == 0 {
		return types.ActionContinue
	}
	if h.responseStreaming {
		return h.handleStreamingUsage(bodySize, endOfStream)
	}
	if !endOfStream {
		return types.ActionContinue
	}

	body, err := proxywasm.GetHttpResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("read token quota response body failed: %v", err)
		h.clearResponse()
		return types.ActionContinue
	}
	tokens, found, err := usage.ParseJSON(body)
	return h.finishUsage(tokens, found, err)
}

// OnHttpStreamDone 关闭当前 HTTP context 的 Redis callback 生命周期
func (h *httpContext) OnHttpStreamDone() {
	redisabi.CloseHTTPContext(h.plugin.contextID, h.contextID)
}

func (h *httpContext) dispatchNextCheck() bool {
	for h.nextCheck < len(h.checks) {
		index := h.nextCheck
		check := h.checks[index]
		h.nextCheck++

		window, err := quotaRedis.NewWindow(check.RedisKey, check.Policy.Quota)
		if err != nil {
			h.checkOutcomes[index].Err = err
			proxywasm.LogErrorf("prepare token quota Redis check failed: %v", err)
			continue
		}
		command, err := window.CheckCommand()
		if err != nil {
			h.checkOutcomes[index].Err = err
			proxywasm.LogErrorf("encode token quota Redis check failed: %v", err)
			continue
		}
		_, err = redisabi.Dispatch(h.plugin.contextID, h.contextID, command, func(result redisabi.Result) {
			h.handleCheckResponse(index, result)
		})
		if err != nil {
			h.checkOutcomes[index].Err = err
			proxywasm.LogErrorf("dispatch token quota Redis check failed: %v", err)
			continue
		}
		return true
	}
	return false
}

func (h *httpContext) handleCheckResponse(index int, result redisabi.Result) {
	outcome := decodeCheckOutcome(result)
	h.checkOutcomes[index] = outcome
	if outcome.Err != nil {
		proxywasm.LogErrorf("complete token quota Redis check failed: %v", outcome.Err)
	}
	if h.dispatchNextCheck() {
		return
	}

	decision := policy.Decide(h.checks, h.checkOutcomes)
	if !decision.Allowed {
		h.clearChecks()
		if err := sendPausedDecision(h.contextID, decision); err != nil {
			proxywasm.LogErrorf("send token quota response failed: %v", err)
		}
		return
	}
	h.bookings = h.checks
	h.clearChecks()
	if err := redisabi.ResumeHTTPRequest(h.contextID); err != nil {
		proxywasm.LogErrorf("resume token quota request failed: %v", err)
	}
}

func decodeCheckOutcome(result redisabi.Result) policy.CheckOutcome {
	if result.Err != nil {
		return policy.CheckOutcome{Err: result.Err}
	}
	if result.Status != redisabi.RedisStatusOK {
		return policy.CheckOutcome{Err: fmt.Errorf("redis call failed with status %d", result.Status)}
	}
	state, err := quotaRedis.ParseCheckState(result.Data)
	if err != nil {
		return policy.CheckOutcome{Err: err}
	}
	return policy.CheckOutcome{
		Allowed:      state.Allowed,
		Used:         state.Used,
		Limit:        state.Limit,
		ResetSeconds: state.ResetSeconds,
	}
}

func (h *httpContext) handleStreamingUsage(bodySize int, endOfStream bool) types.Action {
	var body []byte
	if bodySize > 0 {
		var err error
		body, err = proxywasm.GetHttpResponseBody(0, bodySize)
		if err != nil {
			proxywasm.LogErrorf("read token quota SSE body failed: %v", err)
			h.clearResponse()
			return types.ActionContinue
		}
	}
	if err := h.streamUsage.Push(body); err != nil {
		proxywasm.LogErrorf("parse token quota SSE usage failed: %v", err)
		h.clearResponse()
		return types.ActionContinue
	}
	if h.streamUsage.Complete() {
		tokens, found := h.streamUsage.TotalTokens()
		return h.finishUsage(tokens, found, nil)
	}
	if !endOfStream {
		return types.ActionContinue
	}
	if err := h.streamUsage.Finish(); err != nil {
		proxywasm.LogErrorf("finish token quota SSE usage failed: %v", err)
		h.clearResponse()
		return types.ActionContinue
	}
	tokens, found := h.streamUsage.TotalTokens()
	return h.finishUsage(tokens, found, nil)
}

func (h *httpContext) finishUsage(tokens int64, found bool, parseErr error) types.Action {
	h.responseActive = false
	h.responseStreaming = false
	h.streamUsage = usage.Stream{}
	if parseErr != nil {
		proxywasm.LogErrorf("extract token quota usage failed: %v", parseErr)
		h.bookings = nil
		return types.ActionContinue
	}
	if !found {
		proxywasm.LogError("token quota response does not contain usage")
		h.bookings = nil
		return types.ActionContinue
	}
	if tokens <= 0 {
		h.bookings = nil
		return types.ActionContinue
	}

	h.bookingTokens = tokens
	h.nextBooking = 0
	if h.dispatchNextBooking() {
		return types.ActionPause
	}
	h.clearBookings()
	return types.ActionContinue
}

func (h *httpContext) dispatchNextBooking() bool {
	for h.nextBooking < len(h.bookings) {
		index := h.nextBooking
		booking := h.bookings[index]
		h.nextBooking++

		window, err := quotaRedis.NewWindow(booking.RedisKey, booking.Policy.Quota)
		if err != nil {
			proxywasm.LogErrorf("prepare token quota Redis usage failed: %v", err)
			continue
		}
		command, err := window.AddCommand(h.bookingTokens)
		if err != nil {
			proxywasm.LogErrorf("encode token quota Redis usage failed: %v", err)
			continue
		}
		_, err = redisabi.Dispatch(h.plugin.contextID, h.contextID, command, func(result redisabi.Result) {
			h.handleBookingResponse(index, result)
		})
		if err != nil {
			proxywasm.LogErrorf("dispatch token quota Redis usage failed: %v", err)
			continue
		}
		return true
	}
	return false
}

func (h *httpContext) handleBookingResponse(index int, result redisabi.Result) {
	if result.Err != nil {
		proxywasm.LogErrorf("complete token quota Redis usage for policy %q failed: %v", h.bookings[index].Policy.Name, result.Err)
	} else if result.Status != redisabi.RedisStatusOK {
		proxywasm.LogErrorf("complete token quota Redis usage for policy %q failed with status %d", h.bookings[index].Policy.Name, result.Status)
	} else if _, err := quotaRedis.ParseAddedUsage(result.Data); err != nil {
		proxywasm.LogErrorf("decode token quota Redis usage for policy %q failed: %v", h.bookings[index].Policy.Name, err)
	}

	if h.dispatchNextBooking() {
		return
	}
	h.clearBookings()
	if err := redisabi.ResumeHTTPResponse(h.contextID); err != nil {
		proxywasm.LogErrorf("resume token quota response failed: %v", err)
	}
}

func (h *httpContext) clearChecks() {
	h.checks = nil
	h.checkOutcomes = nil
	h.nextCheck = 0
}

func (h *httpContext) clearBookings() {
	h.bookings = nil
	h.nextBooking = 0
	h.bookingTokens = 0
}

func (h *httpContext) clearResponse() {
	h.responseActive = false
	h.responseStreaming = false
	h.streamUsage = usage.Stream{}
	h.clearBookings()
}

func sendDecision(decision policy.Decision) {
	statusCode, headers, body := decisionResponse(decision)
	pluginwasm.SendResponse(statusCode, headers, body)
}

func sendPausedDecision(contextID uint32, decision policy.Decision) error {
	statusCode, headers, body := decisionResponse(decision)
	return redisabi.SendHTTPResponse(contextID, statusCode, headers, body)
}

func decisionResponse(decision policy.Decision) (int, map[string]string, string) {
	headers := map[string]string{contentTypeHeader: jsonContentType}
	if decision.RetryAfter > 0 {
		headers[retryAfterHeader] = strconv.FormatInt(decision.RetryAfter, 10)
	}
	body := openai.EncodeError(openai.ErrorDetail{
		Message: decision.Message,
		Type:    decision.ErrorType,
		Code:    decision.ErrorCode,
	})
	return decision.StatusCode, headers, string(body)
}
