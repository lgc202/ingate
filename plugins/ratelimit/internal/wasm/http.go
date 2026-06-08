package wasm

import (
	"errors"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	ratelimitruntime "github.com/lgc202/ingate/plugins/ratelimit/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

var errDataPlaneResultCountMismatch = errors.New("rate-limit dataplane response result count mismatch")

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

func (h *httpContext) applyRuntimeResult(result ratelimitruntime.Result) types.Action {
	for _, err := range result.Errors {
		proxywasm.LogErrorf("rate-limit local rule evaluation failed: %v", err)
	}
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}
	if len(result.GlobalChecks) > 0 {
		return h.dispatchGlobalChecks(result.GlobalChecks)
	}
	return proxyWasmAction(result.Action)
}

func (h *httpContext) dispatchGlobalChecks(checks []policy.GlobalCheck) types.Action {
	err := h.plugin.runtime.DispatchGlobalChecks(checks, func(response dataplaneratelimit.CheckResponse, err error) {
		h.handleDataPlaneResponse(checks, response, err)
	})
	if err != nil {
		proxywasm.LogErrorf("dispatch rate-limit dataplane request failed: %v", err)
		return h.handleDataPlaneFailure(checks, err)
	}
	return types.ActionPause
}

func (h *httpContext) handleDataPlaneResponse(checks []policy.GlobalCheck, response dataplaneratelimit.CheckResponse, err error) {
	if err != nil {
		proxywasm.LogErrorf("rate-limit dataplane response failed: %v", err)
	} else if len(response.Results) != len(checks) {
		proxywasm.LogErrorf("rate-limit dataplane response result count mismatch")
		err = errDataPlaneResultCountMismatch
	}

	result := h.plugin.runtime.CompleteGlobalChecks(checks, response, err)
	if result.Action.Kind == pluginruntime.ActionRespond {
		_ = proxyWasmAction(result.Action)
		return
	}
	h.applyQuotaHeaders(result)
	_ = proxywasm.ResumeHttpRequest()
}

func (h *httpContext) handleDataPlaneFailure(checks []policy.GlobalCheck, err error) types.Action {
	if errors.Is(err, ratelimitruntime.ErrDataPlaneUnavailable) {
		proxywasm.LogError("rate-limit dataplane is not configured")
	} else {
		proxywasm.LogErrorf("rate-limit dataplane failed: %v", err)
	}
	result := h.plugin.runtime.CompleteGlobalChecks(checks, dataplaneratelimit.CheckResponse{}, ratelimitruntime.ErrDataPlaneUnavailable)
	return h.applyRuntimeResult(result)
}

func (h *httpContext) applyQuotaHeaders(result ratelimitruntime.Result) {
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}
}
