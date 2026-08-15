package wasm

import (
	"fmt"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"

	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/redis"
)

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok || len(route.config.Policies) == 0 {
		return types.ActionContinue
	}

	checks := policy.BuildChecks(route.config, requestFromProxyWasm(route))
	if len(checks) == 0 {
		return types.ActionContinue
	}
	h.checks = checks
	h.checkOutcomes = make([]policy.CheckOutcome, len(checks))
	h.nextCheck = 0

	if h.dispatchNextCheck() {
		return types.ActionPause
	}
	decision := policy.Decide(h.checks, h.checkOutcomes)
	h.clearChecks()
	if !decision.Allowed {
		pluginwasm.SendResponse(decision.StatusCode, decision.QuotaHeaders, decision.Message)
		return types.ActionPause
	}
	h.quotaHeaders = decision.QuotaHeaders
	return types.ActionContinue
}

func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	pluginwasm.ReplaceResponseHeaders(h.quotaHeaders)
	return types.ActionContinue
}

func (h *httpContext) OnHttpStreamDone() {
	redisabi.CloseHTTPContext(h.plugin.contextID, h.contextID)
}

func (h *httpContext) dispatchNextCheck() bool {
	for h.nextCheck < len(h.checks) {
		index := h.nextCheck
		check := h.checks[index]
		bucket, err := redis.NewTokenBucket(check.RedisKey, check.Rule.Limit)
		if err != nil {
			h.checkOutcomes[index].Err = err
			h.nextCheck++
			proxywasm.LogErrorf("prepare rate-limit Redis check failed: %v", err)
			continue
		}
		command, err := bucket.Command()
		if err != nil {
			h.checkOutcomes[index].Err = err
			h.nextCheck++
			proxywasm.LogErrorf("encode rate-limit Redis check failed: %v", err)
			continue
		}
		h.nextCheck++

		_, err = redisabi.Dispatch(h.plugin.contextID, h.contextID, command, func(result redisabi.Result) {
			h.handleRedisResponse(index, result)
		})
		if err != nil {
			h.checkOutcomes[index].Err = err
			proxywasm.LogErrorf("dispatch rate-limit Redis check failed: %v", err)
			continue
		}
		return true
	}
	return false
}

func (h *httpContext) handleRedisResponse(index int, result redisabi.Result) {
	outcome := decodeCheckOutcome(result)
	h.checkOutcomes[index] = outcome
	if outcome.Err != nil {
		proxywasm.LogErrorf("complete rate-limit Redis check failed: %v", outcome.Err)
	}

	if h.dispatchNextCheck() {
		return
	}
	h.finishPausedRequest(policy.Decide(h.checks, h.checkOutcomes))
}

func (h *httpContext) finishPausedRequest(decision policy.Decision) {
	h.clearChecks()
	if !decision.Allowed {
		if err := redisabi.SendHTTPResponse(
			h.contextID,
			decision.StatusCode,
			decision.QuotaHeaders,
			decision.Message,
		); err != nil {
			proxywasm.LogErrorf("send rate-limit response failed: %v", err)
		}
		return
	}

	h.quotaHeaders = decision.QuotaHeaders
	if err := redisabi.ResumeHTTPRequest(h.contextID); err != nil {
		proxywasm.LogErrorf("resume rate-limit request failed: %v", err)
	}
}

func (h *httpContext) clearChecks() {
	h.checks = nil
	h.checkOutcomes = nil
	h.nextCheck = 0
}

func decodeCheckOutcome(result redisabi.Result) policy.CheckOutcome {
	if result.Err != nil {
		return policy.CheckOutcome{Err: result.Err}
	}
	if result.Status != redisabi.RedisStatusOK {
		return policy.CheckOutcome{Err: fmt.Errorf("redis call failed with status %d", result.Status)}
	}
	state, err := redis.ParseBucketState(result.Data)
	if err != nil {
		return policy.CheckOutcome{Err: err}
	}
	return policy.CheckOutcome{
		Allowed:      state.Allowed,
		Limit:        state.Limit,
		Remaining:    state.Remaining,
		ResetSeconds: state.ResetSeconds,
	}
}
