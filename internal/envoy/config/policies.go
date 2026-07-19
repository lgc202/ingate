package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	accessControlHTTPFilterName = "ingate.filters.http.acl"
	accessControlPluginName     = "ingate.acl"
	accessControlPluginPath     = "/opt/ingate/plugins/acl.wasm"
	rateLimitHTTPFilterName     = "ingate.filters.http.ratelimit"
	rateLimitPluginName         = "ingate.ratelimit"
	rateLimitPluginPath         = "/opt/ingate/plugins/ratelimit.wasm"
	wasmRuntime                 = "envoy.wasm.runtime.v8"
	maxPluginInteger            = 1<<31 - 1
	minPolicyResponseStatusCode = 400
	maxPolicyResponseStatusCode = 599
)

type compiledAccessControlPolicy struct {
	policy  pluginacl.Policy
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

type wasmHTTPFilterOptions struct {
	allowOnHeadersStopIteration bool
}

func (c *compileContext) buildPolicyConfigs() map[listenerKey]listenerPluginConfig {
	// Compiler 已经把 Gateway/Route 应用范围展开成最终执行清单，Wasm 不再理解用户层绑定模型
	accessControlPolicies := c.compileAccessControlPolicies()
	rateLimitPolicies := c.compileRateLimitPolicies()
	result := make(map[listenerKey]listenerPluginConfig)

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
		aclPolicies := make([]pluginacl.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(accessControlPolicies)) {
			compiled := accessControlPolicies[policyID]
			_, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.markProgrammedPolicyTargets(gatewayv1.KindAccessControlPolicy, policyID, matchedTargets)
			aclPolicies = append(aclPolicies, compiled.policy)
		}

		ratePolicies := make([]pluginratelimit.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(rateLimitPolicies)) {
			compiled := rateLimitPolicies[policyID]
			scope, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.markProgrammedPolicyTargets(gatewayv1.KindRateLimitPolicy, policyID, matchedTargets)
			policy := compiled.policy
			policy.Scope = scope
			ratePolicies = append(ratePolicies, policy)
		}

		config := result[key.listenerKey]
		if len(aclPolicies) > 0 {
			if config.accessControl == nil {
				config.accessControl = &pluginacl.PluginConfig{}
			}
			config.accessControl.Routes = append(config.accessControl.Routes, pluginacl.RouteConfig{
				GatewayName: key.gatewayID,
				RouteName:   key.routeID,
				Policies:    aclPolicies,
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
		if len(aclPolicies) > 0 || len(ratePolicies) > 0 {
			result[key.listenerKey] = config
		}
	}

	return result
}

func (c *compileContext) compileAccessControlPolicies() map[string]compiledAccessControlPolicy {
	result := make(map[string]compiledAccessControlPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.accessControlPolicies)) {
		policy := c.accessControlPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindAccessControlPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		pluginPolicy, valid := c.accessControlPolicy(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledAccessControlPolicy{
			policy:  pluginPolicy,
			targets: targets,
		}
	}
	return result
}

func (c *compileContext) compileRateLimitPolicies() map[string]compiledRateLimitPolicy {
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

func (c *compileContext) validPolicyTargets(
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

func (c *compileContext) markProgrammedPolicyTargets(
	policyKind gatewayv1.Kind,
	policyName string,
	targets []gatewayv1.PolicyTargetRef,
) {
	var policy ResourceGeneration
	if policyKind == gatewayv1.KindRateLimitPolicy {
		resource := c.rateLimitPolicies[policyName]
		policy = newResourceGeneration(policyKind, resource.Name, resource.UID, resource.Generation)
	} else {
		resource := c.accessControlPolicies[policyName]
		policy = newResourceGeneration(policyKind, resource.Name, resource.UID, resource.Generation)
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
		c.policyTargets[ProgrammedPolicyTarget{
			Policy: policy,
			Target: targetResource,
		}] = true
	}
}

func (c *compileContext) programmedPolicyTargets() []ProgrammedPolicyTarget {
	result := slices.Collect(maps.Keys(c.policyTargets))
	slices.SortFunc(result, func(a, b ProgrammedPolicyTarget) int {
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

func (c *compileContext) accessControlPolicy(policy *gatewayv1.AccessControlPolicy) (pluginacl.Policy, bool) {
	valid := true
	switch policy.Spec.DefaultAction {
	case "", gatewayv1.AccessControlActionAllow, gatewayv1.AccessControlActionDeny:
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("access control policy %q uses unsupported default action %q", policy.Name, policy.Spec.DefaultAction))
		valid = false
	}
	if len(policy.Spec.Rules) == 0 && policy.Spec.DefaultAction != gatewayv1.AccessControlActionDeny {
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q must have a rule or deny by default", policy.Name))
		valid = false
	}

	rules := make([]pluginacl.Rule, 0, len(policy.Spec.Rules))
	seenRules := make(map[string]bool, len(policy.Spec.Rules))
	for _, rule := range policy.Spec.Rules {
		if rule.Name == "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q has a rule without a name", policy.Name))
			valid = false
			continue
		}
		if seenRules[rule.Name] {
			c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonConflict, fmt.Sprintf("access control policy %q has duplicate rule %q", policy.Name, rule.Name))
			valid = false
			continue
		}
		seenRules[rule.Name] = true
		if rule.Action != gatewayv1.AccessControlActionAllow && rule.Action != gatewayv1.AccessControlActionDeny {
			c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("access control policy %q rule %q uses unsupported action %q", policy.Name, rule.Name, rule.Action))
			valid = false
			continue
		}

		conditions := make([]pluginacl.Condition, 0, len(rule.Conditions))
		for _, condition := range rule.Conditions {
			if condition.Value == "" {
				c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q rule %q has a condition without a value", policy.Name, rule.Name))
				valid = false
				continue
			}
			switch condition.Type {
			case gatewayv1.AccessControlConditionTypeIP:
				if condition.Name != "" {
					c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q rule %q IP condition must not declare a name", policy.Name, rule.Name))
					valid = false
					continue
				}
			case gatewayv1.AccessControlConditionTypeHeader:
				if condition.Name == "" {
					c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q rule %q has a Header condition without a name", policy.Name, rule.Name))
					valid = false
					continue
				}
			default:
				c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("access control policy %q rule %q uses unsupported condition type %q", policy.Name, rule.Name, condition.Type))
				valid = false
				continue
			}
			conditions = append(conditions, pluginacl.Condition{
				Type:  pluginacl.ConditionType(condition.Type),
				Name:  condition.Name,
				Value: condition.Value,
			})
		}
		rules = append(rules, pluginacl.Rule{
			Name:       rule.Name,
			Action:     pluginacl.Action(rule.Action),
			Conditions: conditions,
		})
	}
	if policy.Spec.Response.StatusCode != 0 &&
		(policy.Spec.Response.StatusCode < minPolicyResponseStatusCode || policy.Spec.Response.StatusCode > maxPolicyResponseStatusCode) {
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q response status code must be between 400 and 599", policy.Name))
		valid = false
	}
	return pluginacl.Policy{
		Name:          policy.Name,
		DefaultAction: pluginacl.Action(policy.Spec.DefaultAction),
		Rules:         rules,
		Response: pluginacl.Response{
			StatusCode: policy.Spec.Response.StatusCode,
			Message:    policy.Spec.Response.Message,
		},
	}, valid
}

func (c *compileContext) rateLimitPolicy(policy *gatewayv1.RateLimitPolicy) (pluginratelimit.Policy, bool) {
	valid := true
	if len(policy.Spec.Rules) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q must declare at least one rule", policy.Name))
		valid = false
	}

	rules := make([]pluginratelimit.Rule, 0, len(policy.Spec.Rules))
	seenRules := make(map[string]bool, len(policy.Spec.Rules))
	for _, rule := range policy.Spec.Rules {
		if rule.Name == "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has a rule without a name", policy.Name))
			valid = false
			continue
		}
		if seenRules[rule.Name] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonConflict, fmt.Sprintf("rate limit policy %q has duplicate rule %q", policy.Name, rule.Name))
			valid = false
			continue
		}
		seenRules[rule.Name] = true
		if len(rule.Key.Parts) == 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q must declare at least one key part", policy.Name, rule.Name))
			valid = false
			continue
		}

		parts := make([]pluginratelimit.KeyPart, 0, len(rule.Key.Parts))
		for _, part := range rule.Key.Parts {
			if !c.validRateLimitKeyPart(policy.Name, rule.Name, part) {
				valid = false
				continue
			}
			parts = append(parts, pluginratelimit.KeyPart{
				Type: pluginratelimit.KeyType(part.Type),
				Name: part.Name,
			})
		}
		if rule.Limit.Requests <= 0 || rule.Limit.Requests > maxPluginInteger ||
			rule.Limit.WindowSeconds <= 0 || rule.Limit.WindowSeconds > maxPluginInteger ||
			rule.Limit.Burst < 0 || rule.Limit.Burst > maxPluginInteger {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q has invalid quota", policy.Name, rule.Name))
			valid = false
		}
		rules = append(rules, pluginratelimit.Rule{
			Name:  rule.Name,
			Key:   parts,
			Limit: pluginratelimit.Quota(rule.Limit),
		})
	}
	if policy.Spec.Response.StatusCode != 0 &&
		(policy.Spec.Response.StatusCode < minPolicyResponseStatusCode || policy.Spec.Response.StatusCode > maxPolicyResponseStatusCode) {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q response status code must be between 400 and 599", policy.Name))
		valid = false
	}
	switch policy.Spec.FailurePolicy {
	case "", gatewayv1.RateLimitFailurePolicyFailOpen, gatewayv1.RateLimitFailurePolicyFailClose:
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q uses unsupported failure policy %q", policy.Name, policy.Spec.FailurePolicy))
		valid = false
	}
	return pluginratelimit.Policy{
		Name:  policy.Name,
		Rules: rules,
		Response: pluginratelimit.Response{
			StatusCode:         policy.Spec.Response.StatusCode,
			Message:            policy.Spec.Response.Message,
			QuotaHeaderEnabled: policy.Spec.Response.QuotaHeaderEnabled,
		},
		FailurePolicy: pluginratelimit.FailurePolicy(policy.Spec.FailurePolicy),
	}, valid
}

func (c *compileContext) validRateLimitKeyPart(policyID, ruleName string, part gatewayv1.RateLimitKeyPart) bool {
	switch part.Type {
	case gatewayv1.RateLimitKeyTypeHeader,
		gatewayv1.RateLimitKeyTypeQuery,
		gatewayv1.RateLimitKeyTypeCookie:
		if part.Name != "" {
			return true
		}
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policyID, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q key type %q requires a name", policyID, ruleName, part.Type))
		return false
	case gatewayv1.RateLimitKeyTypeIP,
		gatewayv1.RateLimitKeyTypeRoute,
		gatewayv1.RateLimitKeyTypeGateway,
		gatewayv1.RateLimitKeyTypeRouteRule:
		if part.Name != "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policyID, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q key type %q must not declare a name", policyID, ruleName, part.Type))
			return false
		}
		return true
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policyID, ReasonUnsupported, fmt.Sprintf("rate limit policy %q rule %q uses unsupported key type %q", policyID, ruleName, part.Type))
		return false
	}
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

func buildAccessControlHTTPFilter(config *pluginacl.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode access control plugin config: %w", err)
	}
	return buildWasmHTTPFilter(
		accessControlHTTPFilterName,
		accessControlPluginName,
		accessControlPluginPath,
		raw,
		wasmHTTPFilterOptions{},
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
		wasmHTTPFilterOptions{},
	)
}

func buildWasmHTTPFilter(
	filterName string,
	pluginName string,
	pluginPath string,
	raw []byte,
	options wasmHTTPFilterOptions,
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
	if options.allowOnHeadersStopIteration {
		// AI Proxy 必须在 Header 阶段暂停后继续接收 Body；Envoy 默认兼容模式不会回调请求体
		pluginConfig.AllowOnHeadersStopIteration = wrapperspb.Bool(true)
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
