package wasm

import (
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/redis"
	ratelimitruntime "github.com/lgc202/ingate/plugins/ratelimit/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok || len(route.Config.Policies) == 0 {
		return types.ActionContinue
	}

	result := h.plugin.runtime.Apply(route, requestFromProxyWasm(route))
	return h.applyRuntimeResult(result)
}

func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	pluginwasm.ReplaceResponseHeaders(h.quotaHeaders)
	return types.ActionContinue
}

func (h *httpContext) OnHttpStreamDone() {
	redisabi.CloseHTTPContext(h.plugin.contextID, h.contextID)
}

func (h *httpContext) applyRuntimeResult(result ratelimitruntime.Result) types.Action {
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}
	if result.Action.Kind == pluginruntime.ActionRespond {
		return proxyWasmAction(result.Action)
	}
	if len(result.Checks) > 0 {
		return h.dispatchChecks(result.Checks)
	}
	return proxyWasmAction(result.Action)
}

func (h *httpContext) dispatchChecks(checks []policy.Check) types.Action {
	h.checks = checks
	h.outcomes = make([]policy.Outcome, len(checks))
	h.requests = make([]redis.Request, len(checks))
	h.index = 0

	pending, result := h.dispatchNextCheck()
	if pending {
		return types.ActionPause
	}
	h.clearExecution()
	return h.applyRuntimeResult(result)
}

func (h *httpContext) dispatchNextCheck() (bool, ratelimitruntime.Result) {
	for h.index < len(h.checks) {
		index := h.index
		check := h.checks[index]
		request, command, err := h.plugin.runtime.PrepareCheck(check)
		if err != nil {
			h.outcomes[index].Err = err
			h.index++
			proxywasm.LogErrorf("prepare rate-limit Redis check failed: %v", err)
			continue
		}
		h.requests[index] = request
		h.index++

		_, err = redisabi.Dispatch(h.plugin.contextID, h.contextID, command, func(result redisabi.Result) {
			h.handleRedisResponse(index, result)
		})
		if err != nil {
			h.outcomes[index].Err = err
			proxywasm.LogErrorf("dispatch rate-limit Redis check failed: %v", err)
			continue
		}
		return true, ratelimitruntime.Result{}
	}

	return false, h.plugin.runtime.CompleteChecks(h.checks, h.outcomes)
}

func (h *httpContext) handleRedisResponse(index int, result redisabi.Result) {
	outcome := h.plugin.runtime.CompleteCheck(h.requests[index], result)
	h.outcomes[index] = outcome
	if outcome.Err != nil {
		proxywasm.LogErrorf("complete rate-limit Redis check failed: %v", outcome.Err)
	}

	pending, completed := h.dispatchNextCheck()
	if pending {
		return
	}
	h.finishExecution(completed)
}

func (h *httpContext) finishExecution(result ratelimitruntime.Result) {
	h.clearExecution()
	if result.Action.Kind == pluginruntime.ActionRespond {
		if err := redisabi.SendHTTPResponse(
			h.contextID,
			result.Action.StatusCode,
			result.Action.Headers,
			result.Action.Body,
		); err != nil {
			proxywasm.LogErrorf("send rate-limit response failed: %v", err)
		}
		return
	}

	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}
	if err := redisabi.ResumeHTTPRequest(h.contextID); err != nil {
		proxywasm.LogErrorf("resume rate-limit request failed: %v", err)
	}
}

func (h *httpContext) clearExecution() {
	h.checks = nil
	h.outcomes = nil
	h.requests = nil
	h.index = 0
}
