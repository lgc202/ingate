package compiler

import (
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (c *compilation) headerModifier(route *gatewayv1.Route, modifier *gatewayv1.HeaderModifier) ([]*corev3.HeaderValueOption, []string, bool) {
	if modifier == nil {
		return nil, nil, true
	}
	values := make([]*corev3.HeaderValueOption, 0, len(modifier.Set)+len(modifier.Add))
	valid := len(modifier.Set)+len(modifier.Add)+len(modifier.Remove) > 0
	for _, header := range modifier.Set {
		if header.Name == "" || header.Value == "" {
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD))
	}
	for _, header := range modifier.Add {
		if header.Name == "" || header.Value == "" {
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD))
	}
	for _, name := range modifier.Remove {
		if name == "" {
			valid = false
		}
	}
	if !valid {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid header modifier", route.Name))
	}
	return values, slices.Clone(modifier.Remove), valid
}

func headerValueOption(value gatewayv1.HeaderValue, action corev3.HeaderValueOption_HeaderAppendAction) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: value.Name, Value: value.Value},
		AppendAction: action,
	}
}
