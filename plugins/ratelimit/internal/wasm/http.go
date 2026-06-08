package wasm

import (
	"errors"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/dataplane"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

var (
	errDataPlaneUnavailable         = errors.New("rate-limit dataplane unavailable")
	errDataPlaneResultCountMismatch = errors.New("rate-limit dataplane response result count mismatch")
)

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.routeConfig()
	if !ok || len(route.Bindings) == 0 {
		return types.ActionContinue
	}

	result := h.plugin.policy.Apply(route, requestFromProxyWasm(route))
	return h.applyPolicyResult(result)
}

func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	pluginwasm.ReplaceResponseHeaders(h.quotaHeaders)
	return types.ActionContinue
}

func (h *httpContext) applyPolicyResult(result policy.Result) types.Action {
	for _, err := range result.Errors {
		proxywasm.LogErrorf("rate-limit local rule evaluation failed: %v", err)
	}
	if !result.Allowed {
		sendRejected(result.Decision)
		return types.ActionPause
	}
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}

	if len(result.GlobalChecks) > 0 {
		return h.dispatchGlobalChecks(result.GlobalChecks)
	}

	return types.ActionContinue
}

func (h *httpContext) dispatchGlobalChecks(checks []policy.GlobalCheck) types.Action {
	dataPlaneConfig := h.plugin.config.DataPlane
	if dataPlaneConfig == nil || dataPlaneConfig.ClusterName == "" || dataPlaneConfig.Path == "" {
		return h.handleDataPlaneFailure(checks, "rate-limit dataplane is not configured")
	}

	client := dataplane.New(*dataPlaneConfig)
	err := client.CheckGlobal(h.plugin.config.RedisStores, checks, func(response dataplaneratelimit.CheckResponse, err error) {
		h.handleDataPlaneResponse(checks, response, err)
	})
	if err != nil {
		proxywasm.LogErrorf("dispatch rate-limit dataplane request failed: %v", err)
		return h.handleDataPlaneFailure(checks, "dispatch dataplane request failed")
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

	decision, rejected := policy.ApplyGlobalResult(checks, response, err)
	if rejected {
		sendRejected(decision)
		return
	}
	h.applyQuotaHeaders(decision)
	_ = proxywasm.ResumeHttpRequest()
}

func (h *httpContext) handleDataPlaneFailure(checks []policy.GlobalCheck, message string) types.Action {
	proxywasm.LogErrorf("rate-limit dataplane failed: %s", message)
	decision, rejected := policy.ApplyGlobalResult(checks, dataplaneratelimit.CheckResponse{}, errDataPlaneUnavailable)
	if rejected {
		sendRejected(decision)
		return types.ActionPause
	}
	return types.ActionContinue
}

func (h *httpContext) applyQuotaHeaders(decision policy.Decision) {
	if len(decision.QuotaHeaders) > 0 {
		h.quotaHeaders = decision.QuotaHeaders
	}
}
