package compiler

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	ipRestrictionHTTPFilterName = "ingate.filters.http.iprestriction"
	ipRestrictionPluginName     = "ingate.iprestriction"
	ipRestrictionPluginPath     = "/opt/ingate/plugins/iprestriction.wasm"
	rateLimitHTTPFilterName     = "ingate.filters.http.ratelimit"
	rateLimitPluginName         = "ingate.ratelimit"
	rateLimitPluginPath         = "/opt/ingate/plugins/ratelimit.wasm"
	wasmRuntime                 = "envoy.wasm.runtime.v8"
	rateLimitRuleName           = "limit"
)

type compiledIPRestrictionPolicy struct {
	policy  pluginiprestriction.Policy
	targets []gatewayv1.PolicyTargetRef
}

type compiledRateLimitPolicy struct {
	policy  pluginratelimit.Policy
	targets []gatewayv1.PolicyTargetRef
}

type policyRouteKey struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
}

func (c *compilation) buildPolicyConfigs() map[listenerKey]listenerFilterConfig {
	// 编译阶段已经把 Gateway/Route 应用范围展开成最终执行清单，Wasm 不再理解用户层绑定模型
	ipRestrictionPolicies := c.compileIPRestrictionPolicies()
	rateLimitPolicies := c.compileRateLimitPolicies()
	result := make(map[listenerKey]listenerFilterConfig)

	routeKeySet := make(map[policyRouteKey]bool)
	for _, attachment := range c.routeAttachments {
		routeKeySet[policyRouteKey{
			listenerKey: attachment.listenerKey,
			gatewayID:   attachment.gatewayID,
			routeID:     attachment.routeID,
		}] = true
	}
	routeKeys := slices.Collect(maps.Keys(routeKeySet))
	slices.SortFunc(routeKeys, comparePolicyRouteKeys)

	for _, key := range routeKeys {
		restrictionPolicies := make([]pluginiprestriction.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(ipRestrictionPolicies)) {
			compiled := ipRestrictionPolicies[policyID]
			_, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.recordPolicyTargets(gatewayv1.KindIPRestrictionPolicy, policyID, matchedTargets)
			restrictionPolicies = append(restrictionPolicies, compiled.policy)
		}

		ratePolicies := make([]pluginratelimit.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(rateLimitPolicies)) {
			compiled := rateLimitPolicies[policyID]
			scope, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.recordPolicyTargets(gatewayv1.KindRateLimitPolicy, policyID, matchedTargets)
			policy := compiled.policy
			policy.Scope = scope
			ratePolicies = append(ratePolicies, policy)
		}

		config := result[key.listenerKey]
		if len(restrictionPolicies) > 0 {
			if config.ipRestriction == nil {
				config.ipRestriction = &pluginiprestriction.PluginConfig{}
			}
			config.ipRestriction.Routes = append(config.ipRestriction.Routes, pluginiprestriction.RouteConfig{
				GatewayName: key.gatewayID,
				RouteName:   key.routeID,
				Policies:    restrictionPolicies,
			})
		}
		if len(ratePolicies) > 0 {
			if config.rateLimit == nil {
				config.rateLimit = &pluginratelimit.PluginConfig{}
			}
			config.rateLimit.Routes = append(config.rateLimit.Routes, pluginratelimit.RouteConfig{
				GatewayName: key.gatewayID,
				RouteName:   key.routeID,
				Policies:    ratePolicies,
			})
		}
		if len(restrictionPolicies) > 0 || len(ratePolicies) > 0 {
			result[key.listenerKey] = config
		}
	}
	return result
}

func (c *compilation) compileIPRestrictionPolicies() map[string]compiledIPRestrictionPolicy {
	result := make(map[string]compiledIPRestrictionPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.ipRestrictionPolicies)) {
		policy := c.ipRestrictionPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindIPRestrictionPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		pluginPolicy, valid := c.ipRestrictionPolicy(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledIPRestrictionPolicy{
			policy:  pluginPolicy,
			targets: targets,
		}
	}
	return result
}

func (c *compilation) compileRateLimitPolicies() map[string]compiledRateLimitPolicy {
	result := make(map[string]compiledRateLimitPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.rateLimitPolicies)) {
		policy := c.rateLimitPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindRateLimitPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		pluginPolicy, valid := c.rateLimitPolicy(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledRateLimitPolicy{
			policy:  pluginPolicy,
			targets: targets,
		}
	}
	return result
}

func (c *compilation) validPolicyTargets(
	policyKind gatewayv1.Kind,
	policyID string,
	targets []gatewayv1.PolicyTargetRef,
) []gatewayv1.PolicyTargetRef {
	validTargets := make([]gatewayv1.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		key := string(target.Kind) + "\x00" + target.Name
		if seen[key] {
			c.addDiagnostic(
				SeverityError,
				policyKind,
				policyID,
				ReasonConflict,
				fmt.Sprintf("policy %q repeats target %s %q", policyID, target.Kind, target.Name),
			)
			continue
		}
		seen[key] = true

		if target.Name == "" {
			c.addDiagnostic(SeverityError, policyKind, policyID, ReasonInvalidSpec, fmt.Sprintf("policy %q has a target without a name", policyID))
			continue
		}
		switch target.Kind {
		case gatewayv1.KindGateway:
			if _, exists := c.gateways[target.Name]; !exists {
				c.addDiagnostic(
					SeverityWarning,
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing gateway %q", policyID, target.Name),
				)
				continue
			}
		case gatewayv1.KindRoute:
			if _, exists := c.routes[target.Name]; !exists {
				c.addDiagnostic(
					SeverityWarning,
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing route %q", policyID, target.Name),
				)
				continue
			}
		default:
			c.addDiagnostic(
				SeverityError,
				policyKind,
				policyID,
				ReasonUnsupported,
				fmt.Sprintf("policy %q targets unsupported kind %q", policyID, target.Kind),
			)
			continue
		}
		validTargets = append(validTargets, target)
	}
	return validTargets
}

func matchingPolicyTargets(targets []gatewayv1.PolicyTargetRef, key policyRouteKey) (string, []gatewayv1.PolicyTargetRef) {
	// 同一策略同时命中 Gateway 和 Route 时只执行一次，并让更精确的 Route 范围决定计数作用域
	matched := make([]gatewayv1.PolicyTargetRef, 0, 2)
	var routeTarget *gatewayv1.PolicyTargetRef
	var gatewayTarget *gatewayv1.PolicyTargetRef
	for _, target := range targets {
		switch {
		case target.Kind == gatewayv1.KindRoute && target.Name == key.routeID:
			matched = append(matched, target)
			current := target
			routeTarget = &current
		case target.Kind == gatewayv1.KindGateway && target.Name == key.gatewayID:
			matched = append(matched, target)
			current := target
			gatewayTarget = &current
		}
	}
	if routeTarget != nil {
		return policyScope(*routeTarget), matched
	}
	if gatewayTarget != nil {
		return policyScope(*gatewayTarget), matched
	}
	return "", nil
}

func policyScope(target gatewayv1.PolicyTargetRef) string {
	return string(target.Kind) + "/" + target.Name
}

func (c *compilation) recordPolicyTargets(
	policyKind gatewayv1.Kind,
	policyName string,
	targets []gatewayv1.PolicyTargetRef,
) {
	var policy ResourceGeneration
	switch policyKind {
	case gatewayv1.KindRateLimitPolicy:
		resource := c.rateLimitPolicies[policyName]
		policy = newResourceGeneration(policyKind, resource.Name, resource.UID, resource.Generation)
	case gatewayv1.KindIPRestrictionPolicy:
		resource := c.ipRestrictionPolicies[policyName]
		policy = newResourceGeneration(policyKind, resource.Name, resource.UID, resource.Generation)
	default:
		return
	}

	for _, target := range targets {
		var targetResource ResourceGeneration
		if target.Kind == gatewayv1.KindGateway {
			resource := c.gateways[target.Name]
			targetResource = newResourceGeneration(target.Kind, resource.Name, resource.UID, resource.Generation)
		} else {
			resource := c.routes[target.Name]
			targetResource = newResourceGeneration(target.Kind, resource.Name, resource.UID, resource.Generation)
		}
		c.policyTargetSet[CompiledPolicyTarget{
			Policy: policy,
			Target: targetResource,
		}] = true
	}
}

func (c *compilation) compiledPolicyTargets() []CompiledPolicyTarget {
	result := slices.Collect(maps.Keys(c.policyTargetSet))
	slices.SortFunc(result, func(a, b CompiledPolicyTarget) int {
		if result := compareResourceGeneration(a.Policy, b.Policy); result != 0 {
			return result
		}
		return compareResourceGeneration(a.Target, b.Target)
	})
	return result
}

func compareResourceGeneration(a, b ResourceGeneration) int {
	if result := cmp.Compare(a.Kind, b.Kind); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	if result := cmp.Compare(string(a.UID), string(b.UID)); result != 0 {
		return result
	}
	return cmp.Compare(a.Generation, b.Generation)
}

func (c *compilation) ipRestrictionPolicy(policy *gatewayv1.IPRestrictionPolicy) (pluginiprestriction.Policy, bool) {
	valid := true
	if (len(policy.Spec.Allow) > 0) == (len(policy.Spec.Deny) > 0) {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q must configure exactly one of allow or deny", policy.Name),
		)
		valid = false
	}
	for _, value := range append(slices.Clone(policy.Spec.Allow), policy.Spec.Deny...) {
		if _, err := netip.ParsePrefix(value); err == nil {
			continue
		}
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q contains invalid IP prefix %q", policy.Name, value),
		)
		valid = false
	}
	return pluginiprestriction.Policy{
		Name:  policy.Name,
		Allow: slices.Clone(policy.Spec.Allow),
		Deny:  slices.Clone(policy.Spec.Deny),
	}, valid
}

func (c *compilation) rateLimitPolicy(policy *gatewayv1.RateLimitPolicy) (pluginratelimit.Policy, bool) {
	valid := true
	var key pluginratelimit.KeyPart
	switch policy.Spec.Subject.Type {
	case gatewayv1.RateLimitSubjectShared:
		// Scope 已区分 Gateway 或 Route，Gateway 维度只提供一个稳定的共享 key 值
		key.Type = pluginratelimit.KeyTypeGateway
	case gatewayv1.RateLimitSubjectIP:
		key.Type = pluginratelimit.KeyTypeIP
	case gatewayv1.RateLimitSubjectHeader:
		key.Type = pluginratelimit.KeyTypeHeader
		key.Name = policy.Spec.Subject.HeaderName
		if key.Name == "" || len(k8svalidation.IsHTTPHeaderName(key.Name)) > 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has an invalid subject header name", policy.Name))
			valid = false
		}
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q uses unsupported subject type %q", policy.Name, policy.Spec.Subject.Type))
		valid = false
	}
	if policy.Spec.Subject.Type != gatewayv1.RateLimitSubjectHeader && policy.Spec.Subject.HeaderName != "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q must not declare a subject header name", policy.Name))
		valid = false
	}
	if policy.Spec.Limit.Requests <= 0 || policy.Spec.Limit.Requests > gatewayv1.RateLimitMaxRequests ||
		policy.Spec.Limit.WindowSeconds <= 0 || policy.Spec.Limit.WindowSeconds > gatewayv1.RateLimitMaxWindowSeconds {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has an invalid limit", policy.Name))
		valid = false
	}

	return pluginratelimit.Policy{
		Name: policy.Name,
		Rules: []pluginratelimit.Rule{{
			Name: rateLimitRuleName,
			Key:  []pluginratelimit.KeyPart{key},
			Limit: pluginratelimit.Quota{
				Requests:      int(policy.Spec.Limit.Requests),
				WindowSeconds: int(policy.Spec.Limit.WindowSeconds),
			},
		}},
		Response: pluginratelimit.Response{
			QuotaHeaderEnabled: true,
		},
		FailurePolicy: pluginratelimit.FailurePolicyFailOpen,
	}, valid
}

func comparePolicyRouteKeys(a, b policyRouteKey) int {
	if result := compareListenerKeys(a.listenerKey, b.listenerKey); result != 0 {
		return result
	}
	if a.gatewayID != b.gatewayID {
		if a.gatewayID < b.gatewayID {
			return -1
		}
		return 1
	}
	if a.routeID < b.routeID {
		return -1
	}
	if a.routeID > b.routeID {
		return 1
	}
	return 0
}

func buildIPRestrictionHTTPFilter(config *pluginiprestriction.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode IP restriction plugin config: %w", err)
	}
	return buildWasmHTTPFilter(
		ipRestrictionHTTPFilterName,
		ipRestrictionPluginName,
		ipRestrictionPluginPath,
		raw,
	)
}

func buildRateLimitHTTPFilter(config *pluginratelimit.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode rate limit plugin config: %w", err)
	}
	return buildWasmHTTPFilter(
		rateLimitHTTPFilterName,
		rateLimitPluginName,
		rateLimitPluginPath,
		raw,
	)
}

func buildWasmHTTPFilter(
	filterName string,
	pluginName string,
	pluginPath string,
	raw []byte,
) (*hcmv3.HttpFilter, error) {
	configuration, err := anypb.New(&wrapperspb.StringValue{Value: string(raw)})
	if err != nil {
		return nil, fmt.Errorf("encode Wasm plugin configuration: %w", err)
	}
	pluginConfig := &wasmv3.PluginConfig{
		Name:          pluginName,
		RootId:        pluginName,
		Configuration: configuration,
		FailurePolicy: wasmv3.FailurePolicy_FAIL_CLOSED,
		Vm: &wasmv3.PluginConfig_VmConfig{
			VmConfig: &wasmv3.VmConfig{
				VmId:    pluginName,
				Runtime: wasmRuntime,
				Code: &corev3.AsyncDataSource{
					Specifier: &corev3.AsyncDataSource_Local{
						Local: &corev3.DataSource{
							Specifier: &corev3.DataSource_Filename{
								Filename: pluginPath,
							},
						},
					},
				},
			},
		},
	}
	wasmConfig := &httpwasmv3.Wasm{Config: pluginConfig}
	if err := wasmConfig.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Wasm HTTP filter: %w", err)
	}
	typedConfig, err := anypb.New(wasmConfig)
	if err != nil {
		return nil, fmt.Errorf("encode Wasm HTTP filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name: filterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}
