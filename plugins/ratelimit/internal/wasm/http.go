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
	if !ok || len(route.Config.Bindings) == 0 {
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
	for _, err := range result.Errors {
		proxywasm.LogErrorf("rate-limit local rule evaluation failed: %v", err)
	}
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}
	if result.Action.Kind == pluginruntime.ActionRespond {
		return proxyWasmAction(result.Action)
	}
	if len(result.GlobalChecks) > 0 {
		return h.dispatchGlobalChecks(result.GlobalChecks)
	}
	return proxyWasmAction(result.Action)
}

func (h *httpContext) dispatchGlobalChecks(checks []policy.GlobalCheck) types.Action {
	h.globalChecks = checks
	h.globalOutcomes = make([]policy.GlobalOutcome, len(checks))
	h.globalRequests = make([]redis.Request, len(checks))
	h.globalIndex = 0

	pending, result := h.dispatchNextGlobalCheck()
	if pending {
		return types.ActionPause
	}
	h.clearGlobalExecution()
	return h.applyRuntimeResult(result)
}

func (h *httpContext) dispatchNextGlobalCheck() (bool, ratelimitruntime.Result) {
	for h.globalIndex < len(h.globalChecks) {
		index := h.globalIndex
		check := h.globalChecks[index]
		request, command, err := h.plugin.runtime.PrepareGlobalCheck(check)
		if err != nil {
			h.globalOutcomes[index].Err = err
			h.globalIndex++
			proxywasm.LogErrorf("prepare rate-limit Redis check failed: %v", err)
			continue
		}
		h.globalRequests[index] = request
		h.globalIndex++

		_, err = redisabi.Dispatch(h.plugin.contextID, h.contextID, command, func(result redisabi.Result) {
			h.handleRedisResponse(index, result)
		})
		if err != nil {
			h.globalOutcomes[index].Err = err
			proxywasm.LogErrorf("dispatch rate-limit Redis check failed: %v", err)
			continue
		}
		return true, ratelimitruntime.Result{}
	}

	return false, h.plugin.runtime.CompleteGlobalChecks(h.globalChecks, h.globalOutcomes)
}

func (h *httpContext) handleRedisResponse(index int, result redisabi.Result) {
	outcome := h.plugin.runtime.CompleteGlobalCheck(h.globalRequests[index], result)
	h.globalOutcomes[index] = outcome
	if outcome.Err != nil {
		proxywasm.LogErrorf("complete rate-limit Redis check failed: %v", outcome.Err)
	}

	pending, completed := h.dispatchNextGlobalCheck()
	if pending {
		return
	}
	h.finishGlobalExecution(completed)
}

func (h *httpContext) finishGlobalExecution(result ratelimitruntime.Result) {
	h.clearGlobalExecution()
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

func (h *httpContext) clearGlobalExecution() {
	h.globalChecks = nil
	h.globalOutcomes = nil
	h.globalRequests = nil
	h.globalIndex = 0
}
