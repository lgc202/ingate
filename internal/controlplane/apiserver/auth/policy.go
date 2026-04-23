package auth

import (
	"fmt"
	"os"
	"strings"

	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/yaml"
)

const wildcardMatch = "*"

type Policy struct {
	Rules []PolicyRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

type PolicyRule struct {
	Description     string   `json:"description,omitempty" yaml:"description,omitempty"`
	Users           []string `json:"users,omitempty" yaml:"users,omitempty"`
	Groups          []string `json:"groups,omitempty" yaml:"groups,omitempty"`
	Verbs           []string `json:"verbs,omitempty" yaml:"verbs,omitempty"`
	APIGroups       []string `json:"apiGroups,omitempty" yaml:"apiGroups,omitempty"`
	Resources       []string `json:"resources,omitempty" yaml:"resources,omitempty"`
	ResourceNames   []string `json:"resourceNames,omitempty" yaml:"resourceNames,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty" yaml:"nonResourceURLs,omitempty"`
}

func DefaultPolicy(anonymousPaths []string) Policy {
	return Policy{Rules: []PolicyRule{
		{
			Description:     "allow anonymous discovery and health endpoints",
			Groups:          []string{userinfo.AllUnauthenticated},
			Verbs:           []string{"get"},
			NonResourceURLs: publicPathPatterns(anonymousPaths),
		},
		{
			Description: "allow administrators full access",
			Groups:      []string{userinfo.SystemPrivilegedGroup},
			Verbs:       []string{wildcardMatch},
			APIGroups:   []string{wildcardMatch},
			Resources:   []string{wildcardMatch},
		},
		{
			Description:     "allow administrators full non-resource access",
			Groups:          []string{userinfo.SystemPrivilegedGroup},
			Verbs:           []string{wildcardMatch},
			NonResourceURLs: []string{wildcardMatch},
		},
		{
			Description: "allow viewers read-only access to ingate resources",
			Groups:      append([]string(nil), DefaultViewerGroups...),
			Verbs:       []string{"get", "list", "watch"},
			APIGroups:   []string{"gateway.ingate.io", "policy.ingate.io"},
			Resources:   []string{wildcardMatch},
		},
	}}
}

func LoadPolicy(policyFile string, anonymousPaths []string) (Policy, error) {
	if strings.TrimSpace(policyFile) == "" {
		return DefaultPolicy(anonymousPaths), nil
	}

	content, err := os.ReadFile(policyFile)
	if err != nil {
		return Policy{}, fmt.Errorf("read authorization policy file: %w", err)
	}

	policy := Policy{}
	if err := yaml.Unmarshal(content, &policy); err != nil {
		return Policy{}, fmt.Errorf("decode authorization policy file: %w", err)
	}
	if len(policy.Rules) == 0 {
		return Policy{}, fmt.Errorf("authorization policy file contains no rules")
	}

	return policy, nil
}

func publicPathPatterns(paths []string) []string {
	patterns := make([]string, 0, len(paths)*2)
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		patterns = append(patterns, path)
		if path != "/" {
			patterns = append(patterns, strings.TrimRight(path, "/")+"/*")
		}
	}
	return patterns
}

func ruleMatchesUser(rule PolicyRule, user userinfo.Info) bool {
	if user == nil {
		return false
	}
	if len(rule.Users) == 0 && len(rule.Groups) == 0 {
		return false
	}
	if matchExactOrWildcard(rule.Users, user.GetName()) {
		return true
	}
	for _, group := range user.GetGroups() {
		if matchExactOrWildcard(rule.Groups, group) {
			return true
		}
	}
	return false
}

func ruleMatchesAttributes(rule PolicyRule, attrs authorizer.Attributes) bool {
	if !matchExactOrWildcard(rule.Verbs, attrs.GetVerb()) {
		return false
	}

	if attrs.IsResourceRequest() {
		if len(rule.Resources) == 0 {
			return false
		}
		resource := attrs.GetResource()
		if subresource := attrs.GetSubresource(); subresource != "" {
			resource = resource + "/" + subresource
		}
		if !matchExactOrWildcard(rule.APIGroups, attrs.GetAPIGroup()) {
			return false
		}
		if !matchExactOrWildcard(rule.Resources, resource) {
			return false
		}
		if len(rule.ResourceNames) > 0 && !matchExactOrWildcard(rule.ResourceNames, attrs.GetName()) {
			return false
		}
		return true
	}

	if len(rule.NonResourceURLs) == 0 {
		return false
	}
	return matchPathPattern(rule.NonResourceURLs, attrs.GetPath())
}

func matchExactOrWildcard(patterns []string, value string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if pattern == wildcardMatch || pattern == value {
			return true
		}
	}
	return false
}

func matchPathPattern(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if pattern == wildcardMatch || pattern == value {
			return true
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}
