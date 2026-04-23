package auth

import (
	"context"
	"fmt"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type policyAuthorizer struct {
	rules []PolicyRule
}

func NewAuthorizer(anonymousPaths []string, policyFile string) (authorizer.Authorizer, error) {
	policy, err := LoadPolicy(policyFile, anonymousPaths)
	if err != nil {
		return nil, err
	}
	return policyAuthorizer{rules: append([]PolicyRule(nil), policy.Rules...)}, nil
}

func (a policyAuthorizer) Authorize(_ context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	user := attrs.GetUser()
	if user == nil {
		return authorizer.DecisionDeny, "missing user information", nil
	}

	for _, rule := range a.rules {
		if !ruleMatchesUser(rule, user) {
			continue
		}
		if !ruleMatchesAttributes(rule, attrs) {
			continue
		}
		reason := rule.Description
		if reason == "" {
			reason = fmt.Sprintf("matched authorization rule for user %s", user.GetName())
		}
		return authorizer.DecisionAllow, reason, nil
	}

	return authorizer.DecisionDeny, "request is not authorized", nil
}
