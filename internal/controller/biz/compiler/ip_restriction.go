package compiler

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	rbacconfigv3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	httprbacv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/iprestrictionconfig"
)

const (
	ipRestrictionHTTPFilterName = "envoy.filters.http.rbac"
	ipRestrictionRuleName       = "ingate-ip-restriction"
)

type ipRestrictionPolicy struct {
	allow []netip.Prefix
	deny  []netip.Prefix
}

type compiledIPRestrictionPolicy struct {
	source  ResourceGeneration
	policy  ipRestrictionPolicy
	targets []gatewayv1.PolicyTargetRef
}

func (c *compilation) compileIPRestrictionPolicies() []compiledIPRestrictionPolicy {
	result := make([]compiledIPRestrictionPolicy, 0, len(c.ipRestrictionPolicies))
	for _, policyID := range slices.Sorted(maps.Keys(c.ipRestrictionPolicies)) {
		policy := c.ipRestrictionPolicies[policyID]
		targets := c.validPolicyTargets(
			gatewayv1.KindIPRestrictionPolicy,
			policyID,
			policy.Spec.TargetRefs,
			gatewayv1.KindGateway,
			gatewayv1.KindRoute,
		)
		if !policy.Spec.Enabled {
			continue
		}
		compiled, valid := c.ipRestrictionPolicy(policy)
		if !valid {
			continue
		}
		result = append(result, compiledIPRestrictionPolicy{
			source:  newResourceGeneration(gatewayv1.KindIPRestrictionPolicy, policy),
			policy:  compiled,
			targets: targets,
		})
	}
	return result
}

func (c *compilation) ipRestrictionPolicy(policy *gatewayv1.IPRestrictionPolicy) (ipRestrictionPolicy, bool) {
	valid := true
	if (len(policy.Spec.Allow) > 0) == (len(policy.Spec.Deny) > 0) {
		c.addResourceError(
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q must configure exactly one of allow or deny", policy.Name),
		)
		valid = false
	}
	allow, allowValid := c.ipPrefixes(policy, policy.Spec.Allow)
	deny, denyValid := c.ipPrefixes(policy, policy.Spec.Deny)
	return ipRestrictionPolicy{allow: allow, deny: deny}, valid && allowValid && denyValid
}

func (c *compilation) ipPrefixes(policy *gatewayv1.IPRestrictionPolicy, values []string) ([]netip.Prefix, bool) {
	valid := true
	if len(values) > iprestrictionconfig.MaxRanges {
		c.addResourceError(
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q contains too many IP ranges", policy.Name),
		)
		values = values[:iprestrictionconfig.MaxRanges]
		valid = false
	}

	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		normalized, ok := iprestrictionconfig.NormalizeRange(value)
		if !ok {
			c.addResourceError(
				gatewayv1.KindIPRestrictionPolicy,
				policy.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("IP restriction policy %q contains invalid IP prefix %q", policy.Name, value),
			)
			valid = false
			continue
		}
		prefix, _ := netip.ParsePrefix(normalized)
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, valid
}

func buildIPRestrictionHTTPFilter() (*hcmv3.HttpFilter, error) {
	typedConfig, err := anypb.New(&httprbacv3.RBAC{})
	if err != nil {
		return nil, fmt.Errorf("encode Envoy RBAC filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name:       ipRestrictionHTTPFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}

func applyIPRestrictionPolicies(routes []*routev3.Route, policies []ipRestrictionPolicy) error {
	principals := lo.Map(policies, func(policy ipRestrictionPolicy, _ int) *rbacconfigv3.Principal {
		return restrictionPrincipal(policy)
	})
	rules := &httprbacv3.RBAC{Rules: &rbacconfigv3.RBAC{
		Action: rbacconfigv3.RBAC_ALLOW,
		Policies: map[string]*rbacconfigv3.Policy{
			ipRestrictionRuleName: {
				Permissions: []*rbacconfigv3.Permission{{Rule: &rbacconfigv3.Permission_Any{Any: true}}},
				Principals: []*rbacconfigv3.Principal{{Identifier: &rbacconfigv3.Principal_AndIds{
					AndIds: &rbacconfigv3.Principal_Set{Ids: principals},
				}}},
			},
		},
	}}
	perRoute, err := anypb.New(&httprbacv3.RBACPerRoute{Rbac: rules})
	if err != nil {
		return fmt.Errorf("encode Envoy route RBAC config: %w", err)
	}
	for _, route := range routes {
		if route.TypedPerFilterConfig == nil {
			route.TypedPerFilterConfig = make(map[string]*anypb.Any)
		}
		route.TypedPerFilterConfig[ipRestrictionHTTPFilterName] = perRoute
	}
	return nil
}

func restrictionPrincipal(policy ipRestrictionPolicy) *rbacconfigv3.Principal {
	if len(policy.allow) > 0 {
		return anyIPPrincipal(policy.allow)
	}
	return &rbacconfigv3.Principal{Identifier: &rbacconfigv3.Principal_NotId{
		NotId: anyIPPrincipal(policy.deny),
	}}
}

func anyIPPrincipal(prefixes []netip.Prefix) *rbacconfigv3.Principal {
	principals := lo.Map(prefixes, func(prefix netip.Prefix, _ int) *rbacconfigv3.Principal {
		return &rbacconfigv3.Principal{Identifier: &rbacconfigv3.Principal_DirectRemoteIp{
			DirectRemoteIp: &corev3.CidrRange{
				AddressPrefix: prefix.Addr().String(),
				PrefixLen:     wrapperspb.UInt32(uint32(prefix.Bits())),
			},
		}}
	})
	if len(principals) == 1 {
		return principals[0]
	}
	return &rbacconfigv3.Principal{Identifier: &rbacconfigv3.Principal_OrIds{
		OrIds: &rbacconfigv3.Principal_Set{Ids: principals},
	}}
}
