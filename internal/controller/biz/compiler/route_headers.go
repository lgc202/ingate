package compiler

import (
	"cmp"
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func (c *compilation) buildHeaderModifier(
	route *gatewayv1.Route,
	modifier *gatewayv1.HeaderModifier,
) ([]*corev3.HeaderValueOption, []string, bool) {
	if modifier == nil {
		return nil, nil, true
	}
	actionCount := len(modifier.Set) + len(modifier.Add) + len(modifier.Remove)
	if actionCount == 0 || actionCount > routeconfig.MaxHeaderModifierActions {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q has an invalid header modifier", route.Name),
		)
		return nil, nil, false
	}

	headersToAdd := make([]*corev3.HeaderValueOption, 0, len(modifier.Set)+len(modifier.Add))
	headersToRemove := make([]string, 0, len(modifier.Remove))
	usedNames := make(map[string]bool, actionCount)
	valid := true
	for _, header := range normalizedHeaderValues(modifier.Set) {
		if !isValidHeaderValue(header) || usedNames[header.Name] {
			valid = false
			continue
		}
		usedNames[header.Name] = true
		headersToAdd = append(
			headersToAdd,
			headerValueOption(header, corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD),
		)
	}
	for _, header := range normalizedHeaderValues(modifier.Add) {
		if !isValidHeaderValue(header) || usedNames[header.Name] {
			valid = false
			continue
		}
		usedNames[header.Name] = true
		headersToAdd = append(
			headersToAdd,
			headerValueOption(header, corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD),
		)
	}
	for _, name := range normalizedHeaderNames(modifier.Remove) {
		if !httpheader.IsValidName(name) || usedNames[name] {
			valid = false
			continue
		}
		usedNames[name] = true
		headersToRemove = append(headersToRemove, name)
	}
	if !valid {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q has an invalid header modifier", route.Name),
		)
	}
	return headersToAdd, headersToRemove, valid
}

func normalizedHeaderValues(values []gatewayv1.HeaderValue) []gatewayv1.HeaderValue {
	result := slices.Clone(values)
	for i := range result {
		result[i].Name = httpheader.NormalizeName(result[i].Name)
		result[i].Value = httpheader.NormalizeValue(result[i].Value)
	}
	slices.SortFunc(result, func(a, b gatewayv1.HeaderValue) int {
		return cmp.Or(
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Value, b.Value),
		)
	})
	return result
}

func normalizedHeaderNames(names []string) []string {
	result := slices.Clone(names)
	for i := range result {
		result[i] = httpheader.NormalizeName(result[i])
	}
	slices.Sort(result)
	return result
}

func isValidHeaderValue(value gatewayv1.HeaderValue) bool {
	return httpheader.IsValidName(value.Name) &&
		value.Value != "" &&
		httpheader.IsValidValue(value.Value)
}

func headerValueOption(
	value gatewayv1.HeaderValue,
	action corev3.HeaderValueOption_HeaderAppendAction,
) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: value.Name, Value: value.Value},
		AppendAction: action,
	}
}
