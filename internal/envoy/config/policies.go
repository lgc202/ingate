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
	accessControlSchemaVersion  = "v1"
	rateLimitHTTPFilterName     = "ingate.filters.http.ratelimit"
	rateLimitPluginName         = "ingate.ratelimit"
	rateLimitPluginPath         = "/opt/ingate/plugins/ratelimit.wasm"
	wasmRuntime                 = "envoy.wasm.runtime.v8"
)

type listenerPolicyConfig struct {
	accessControl *pluginacl.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
}

type compiledPolicyBinding struct {
	name                  string
	target                gatewayv1.PolicyTargetRef
	accessControlPolicies []pluginacl.Policy
	rateLimitPolicies     []pluginratelimit.Policy
}

type rateLimitRouteKey struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
}

func (c *compileContext) buildPolicyConfigs() map[listenerKey]listenerPolicyConfig {
	// 每个 HCM 只注入一次内置 filter，插件再用当前 xDS route name 定位 Gateway/Route/Rule
	// RateLimit 配置直接使用最终 routes/bindings/policies 结构，系统 Redis 不进入插件 JSON
	bindings := c.compilePolicyBindings()
	result := make(map[listenerKey]listenerPolicyConfig)

	for _, attachment := range c.routeAttachments {
		aclBindings := make([]pluginacl.Binding, 0)
		for _, binding := range bindings {
			if len(binding.accessControlPolicies) == 0 || !bindingMatchesAttachment(binding.target, attachment) {
				continue
			}
			aclBindings = append(aclBindings, pluginacl.Binding{
				Name: binding.name,
				Target: pluginacl.Target{
					Kind:     string(binding.target.Kind),
					Name:     binding.target.Name,
					RuleName: binding.target.RuleName,
				},
				Policies: binding.accessControlPolicies,
			})
		}
		if len(aclBindings) == 0 {
			continue
		}
		config := result[attachment.listenerKey]
		if config.accessControl == nil {
			config.accessControl = &pluginacl.PluginConfig{SchemaVersion: accessControlSchemaVersion}
		}
		config.accessControl.Routes = append(config.accessControl.Routes, pluginacl.RouteConfig{
			SchemaVersion: accessControlSchemaVersion,
			GatewayName:   attachment.gatewayID,
			RouteName:     attachment.routeID,
			RuleName:      attachment.ruleName,
			Bindings:      aclBindings,
		})
		result[attachment.listenerKey] = config
	}

	routeRules := make(map[rateLimitRouteKey]map[string]bool)
	for _, attachment := range c.routeAttachments {
		key := rateLimitRouteKey{
			listenerKey: attachment.listenerKey,
			gatewayID:   attachment.gatewayID,
			routeID:     attachment.routeID,
		}
		if routeRules[key] == nil {
			routeRules[key] = make(map[string]bool)
		}
		routeRules[key][attachment.ruleName] = true
	}
	routeKeys := slices.Collect(maps.Keys(routeRules))
	slices.SortFunc(routeKeys, compareRateLimitRouteKeys)
	for _, key := range routeKeys {
		rateBindings := make([]pluginratelimit.Binding, 0)
		for _, binding := range bindings {
			if len(binding.rateLimitPolicies) == 0 || !bindingMatchesRateLimitRoute(binding.target, key, routeRules[key]) {
				continue
			}
			rateBindings = append(rateBindings, pluginratelimit.Binding{
				Name: binding.name,
				Target: pluginratelimit.Target{
					Kind:     string(binding.target.Kind),
					Name:     binding.target.Name,
					RuleName: binding.target.RuleName,
				},
				Policies: binding.rateLimitPolicies,
			})
		}
		if len(rateBindings) == 0 {
			continue
		}
		config := result[key.listenerKey]
		if config.rateLimit == nil {
			config.rateLimit = &pluginratelimit.PluginConfig{}
		}
		config.rateLimit.Routes = append(config.rateLimit.Routes, pluginratelimit.RouteConfig{
			GatewayName: key.gatewayID,
			RouteName:   key.routeID,
			Bindings:    rateBindings,
		})
		result[key.listenerKey] = config
	}

	return result
}

func (c *compileContext) compilePolicyBindings() []compiledPolicyBinding {
	bindings := make([]compiledPolicyBinding, 0, len(c.policyBindings))
	for _, bindingID := range slices.Sorted(maps.Keys(c.policyBindings)) {
		binding := c.policyBindings[bindingID]
		if !binding.Spec.Enabled {
			continue
		}
		if !c.validatePolicyTarget(binding) {
			continue
		}
		if len(binding.Spec.Policies) == 0 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindPolicyBinding,
				bindingID,
				ReasonInvalidSpec,
				fmt.Sprintf("policy binding %q must reference at least one policy", bindingID),
			)
			continue
		}

		compiled := compiledPolicyBinding{
			name:   bindingID,
			target: binding.Spec.TargetRef,
		}
		policyRefs := slices.Clone(binding.Spec.Policies)
		slices.SortFunc(policyRefs, func(a, b gatewayv1.PolicyRef) int {
			if a.Kind != b.Kind {
				return cmp.Compare(a.Kind, b.Kind)
			}
			return cmp.Compare(a.Name, b.Name)
		})
		seen := make(map[string]bool, len(policyRefs))
		for _, ref := range policyRefs {
			key := string(ref.Kind) + "\x00" + ref.Name
			if seen[key] {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindPolicyBinding,
					bindingID,
					ReasonConflict,
					fmt.Sprintf("policy binding %q repeats policy %s %q", bindingID, ref.Kind, ref.Name),
				)
				continue
			}
			seen[key] = true

			switch ref.Kind {
			case gatewayv1.KindAccessControlPolicy:
				policy, exists := c.accessControlPolicies[ref.Name]
				if !exists {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindPolicyBinding,
						bindingID,
						ReasonReferenceNotFound,
						fmt.Sprintf("policy binding %q references missing access control policy %q", bindingID, ref.Name),
					)
					continue
				}
				if !policy.Spec.Enabled {
					continue
				}
				pluginPolicy, ok := c.accessControlPolicy(policy)
				if ok {
					compiled.accessControlPolicies = append(compiled.accessControlPolicies, pluginPolicy)
				}
			case gatewayv1.KindRateLimitPolicy:
				policy, exists := c.rateLimitPolicies[ref.Name]
				if !exists {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindPolicyBinding,
						bindingID,
						ReasonReferenceNotFound,
						fmt.Sprintf("policy binding %q references missing rate limit policy %q", bindingID, ref.Name),
					)
					continue
				}
				if !policy.Spec.Enabled {
					continue
				}
				pluginPolicy, ok := c.rateLimitPolicy(policy)
				if ok {
					compiled.rateLimitPolicies = append(compiled.rateLimitPolicies, pluginPolicy)
				}
			default:
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindPolicyBinding,
					bindingID,
					ReasonUnsupported,
					fmt.Sprintf("policy binding %q references unsupported policy kind %q", bindingID, ref.Kind),
				)
			}
		}
		if len(compiled.accessControlPolicies) > 0 || len(compiled.rateLimitPolicies) > 0 {
			bindings = append(bindings, compiled)
		}
	}
	return bindings
}

func (c *compileContext) validatePolicyTarget(binding *gatewayv1.PolicyBinding) bool {
	target := binding.Spec.TargetRef
	if target.Name == "" {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindPolicyBinding,
			binding.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("policy binding %q target name is required", binding.Name),
		)
		return false
	}
	switch target.Kind {
	case gatewayv1.KindGateway:
		if target.RuleName != "" {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindPolicyBinding,
				binding.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("policy binding %q cannot set ruleName for a Gateway target", binding.Name),
			)
			return false
		}
		if _, exists := c.gateways[target.Name]; !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindPolicyBinding,
				binding.Name,
				ReasonReferenceNotFound,
				fmt.Sprintf("policy binding %q references missing gateway %q", binding.Name, target.Name),
			)
			return false
		}
	case gatewayv1.KindRoute:
		if _, exists := c.routes[target.Name]; !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindPolicyBinding,
				binding.Name,
				ReasonReferenceNotFound,
				fmt.Sprintf("policy binding %q references missing route %q", binding.Name, target.Name),
			)
			return false
		}
		if target.RuleName != "" && !c.routeRules[target.Name][target.RuleName] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindPolicyBinding,
				binding.Name,
				ReasonReferenceNotFound,
				fmt.Sprintf("policy binding %q references missing route %q rule %q", binding.Name, target.Name, target.RuleName),
			)
			return false
		}
	default:
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindPolicyBinding,
			binding.Name,
			ReasonUnsupported,
			fmt.Sprintf("policy binding %q targets unsupported kind %q", binding.Name, target.Kind),
		)
		return false
	}
	return true
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
			case gatewayv1.AccessControlConditionTypeIP,
				gatewayv1.AccessControlConditionTypeConsumer,
				gatewayv1.AccessControlConditionTypeTenant:
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
	if policy.Spec.Response.StatusCode != 0 && (policy.Spec.Response.StatusCode < 100 || policy.Spec.Response.StatusCode > 599) {
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("access control policy %q response status code must be between 100 and 599", policy.Name))
		valid = false
	}
	return pluginacl.Policy{
		Name:          policy.Name,
		DisplayName:   policy.Spec.DisplayName,
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
	switch policy.Spec.Mode {
	case gatewayv1.RateLimitModeLocal, gatewayv1.RateLimitModeGlobal:
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q uses unsupported mode %q", policy.Name, policy.Spec.Mode))
		valid = false
	}
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
		if rule.Limit.Requests <= 0 || rule.Limit.WindowSeconds <= 0 || rule.Limit.Burst < 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q has invalid quota", policy.Name, rule.Name))
			valid = false
		}
		switch rule.Algorithm {
		case "", gatewayv1.RateLimitAlgorithmFixedWindow, gatewayv1.RateLimitAlgorithmSlidingWindow, gatewayv1.RateLimitAlgorithmTokenBucket:
		default:
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q rule %q uses unsupported algorithm %q", policy.Name, rule.Name, rule.Algorithm))
			valid = false
		}
		rules = append(rules, pluginratelimit.Rule{
			Name:      rule.Name,
			Key:       parts,
			Limit:     pluginratelimit.Quota(rule.Limit),
			Algorithm: pluginratelimit.Algorithm(rule.Algorithm),
		})
	}
	if policy.Spec.Response.StatusCode != 0 && (policy.Spec.Response.StatusCode < 100 || policy.Spec.Response.StatusCode > 599) {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q response status code must be between 100 and 599", policy.Name))
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
		Mode:  pluginratelimit.Mode(policy.Spec.Mode),
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
		gatewayv1.RateLimitKeyTypeCookie,
		gatewayv1.RateLimitKeyTypeJWTClaim:
		if part.Name != "" {
			return true
		}
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policyID, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q rule %q key type %q requires a name", policyID, ruleName, part.Type))
		return false
	case gatewayv1.RateLimitKeyTypeIP,
		gatewayv1.RateLimitKeyTypeConsumer,
		gatewayv1.RateLimitKeyTypeRoute,
		gatewayv1.RateLimitKeyTypeGateway,
		gatewayv1.RateLimitKeyTypeRouteRule,
		gatewayv1.RateLimitKeyTypeAPIKey,
		gatewayv1.RateLimitKeyTypeTenant:
		return true
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policyID, ReasonUnsupported, fmt.Sprintf("rate limit policy %q rule %q uses unsupported key type %q", policyID, ruleName, part.Type))
		return false
	}
}

func bindingMatchesAttachment(target gatewayv1.PolicyTargetRef, attachment routeAttachment) bool {
	switch target.Kind {
	case gatewayv1.KindGateway:
		return target.Name == attachment.gatewayID
	case gatewayv1.KindRoute:
		return target.Name == attachment.routeID && (target.RuleName == "" || target.RuleName == attachment.ruleName)
	default:
		return false
	}
}

func bindingMatchesRateLimitRoute(target gatewayv1.PolicyTargetRef, key rateLimitRouteKey, rules map[string]bool) bool {
	switch target.Kind {
	case gatewayv1.KindGateway:
		return target.Name == key.gatewayID
	case gatewayv1.KindRoute:
		return target.Name == key.routeID && (target.RuleName == "" || rules[target.RuleName])
	default:
		return false
	}
}

func compareRateLimitRouteKeys(a, b rateLimitRouteKey) int {
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

func buildWasmHTTPFilter(filterName, pluginName, pluginPath string, raw []byte) (*hcmv3.HttpFilter, error) {
	configuration, err := anypb.New(&wrapperspb.StringValue{Value: string(raw)})
	if err != nil {
		return nil, fmt.Errorf("encode Wasm plugin configuration: %w", err)
	}
	wasmConfig := &httpwasmv3.Wasm{
		Config: &wasmv3.PluginConfig{
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
		},
	}
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
