package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

const (
	authPolicyKind    = "AuthPolicy"
	trafficPolicyKind = "TrafficPolicy"
)

func AuthPolicyFromCreateRequest(req dto.CreateAuthPolicyRequest) *policyv1alpha1.AuthPolicy {
	return &policyv1alpha1.AuthPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.SchemeGroupVersion.String(),
			Kind:       authPolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: policyv1alpha1.AuthPolicySpec{
			TargetRefs: targetRefsFromDTO(req.TargetRefs),
			Type:       req.Type,
			JWT:        jwtAuthFromDTO(req.JWT),
			APIKey:     apiKeyAuthFromDTO(req.APIKey),
		},
	}
}

func AuthPolicyFromUpdateRequest(name string, req dto.UpdateAuthPolicyRequest) *policyv1alpha1.AuthPolicy {
	return &policyv1alpha1.AuthPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.SchemeGroupVersion.String(),
			Kind:       authPolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policyv1alpha1.AuthPolicySpec{
			TargetRefs: targetRefsFromDTO(req.TargetRefs),
			Type:       req.Type,
			JWT:        jwtAuthFromDTO(req.JWT),
			APIKey:     apiKeyAuthFromDTO(req.APIKey),
		},
	}
}

func AuthPolicyToResponse(policy *policyv1alpha1.AuthPolicy) dto.AuthPolicyResponse {
	if policy == nil {
		return dto.AuthPolicyResponse{}
	}
	return dto.AuthPolicyResponse{
		Metadata: dto.NewObjectMeta(policy.ObjectMeta),
		Spec: dto.AuthPolicySpec{
			TargetRefs: targetRefsToDTO(policy.Spec.TargetRefs),
			Type:       policy.Spec.Type,
			JWT:        jwtAuthToDTO(policy.Spec.JWT),
			APIKey:     apiKeyAuthToDTO(policy.Spec.APIKey),
		},
		Status: dto.AuthPolicyStatusView{
			ObservedGeneration: policy.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(policy.Status.Conditions),
		},
	}
}

func AuthPolicyListToResponse(list *policyv1alpha1.AuthPolicyList) dto.AuthPolicyListResponse {
	if list == nil {
		return dto.AuthPolicyListResponse{}
	}
	items := make([]dto.AuthPolicyResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, AuthPolicyToResponse(&list.Items[i]))
	}
	return dto.AuthPolicyListResponse{Items: items}
}

func TrafficPolicyFromCreateRequest(req dto.CreateTrafficPolicyRequest) *policyv1alpha1.TrafficPolicy {
	return &policyv1alpha1.TrafficPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.SchemeGroupVersion.String(),
			Kind:       trafficPolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: policyv1alpha1.TrafficPolicySpec{
			TargetRefs: targetRefsFromDTO(req.TargetRefs),
			Timeout:    timeoutFromDTO(req.Timeout),
			Retry:      retryFromDTO(req.Retry),
			RateLimit:  rateLimitFromDTO(req.RateLimit),
		},
	}
}

func TrafficPolicyFromUpdateRequest(name string, req dto.UpdateTrafficPolicyRequest) *policyv1alpha1.TrafficPolicy {
	return &policyv1alpha1.TrafficPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.SchemeGroupVersion.String(),
			Kind:       trafficPolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policyv1alpha1.TrafficPolicySpec{
			TargetRefs: targetRefsFromDTO(req.TargetRefs),
			Timeout:    timeoutFromDTO(req.Timeout),
			Retry:      retryFromDTO(req.Retry),
			RateLimit:  rateLimitFromDTO(req.RateLimit),
		},
	}
}

func TrafficPolicyToResponse(policy *policyv1alpha1.TrafficPolicy) dto.TrafficPolicyResponse {
	if policy == nil {
		return dto.TrafficPolicyResponse{}
	}
	return dto.TrafficPolicyResponse{
		Metadata: dto.NewObjectMeta(policy.ObjectMeta),
		Spec: dto.TrafficPolicySpec{
			TargetRefs: targetRefsToDTO(policy.Spec.TargetRefs),
			Timeout:    timeoutToDTO(policy.Spec.Timeout),
			Retry:      retryToDTO(policy.Spec.Retry),
			RateLimit:  rateLimitToDTO(policy.Spec.RateLimit),
		},
		Status: dto.TrafficPolicyStatusView{
			ObservedGeneration: policy.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(policy.Status.Conditions),
		},
	}
}

func TrafficPolicyListToResponse(list *policyv1alpha1.TrafficPolicyList) dto.TrafficPolicyListResponse {
	if list == nil {
		return dto.TrafficPolicyListResponse{}
	}
	items := make([]dto.TrafficPolicyResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, TrafficPolicyToResponse(&list.Items[i]))
	}
	return dto.TrafficPolicyListResponse{Items: items}
}

func targetRefsFromDTO(refs []dto.TargetRef) []policyv1alpha1.TargetReference {
	items := make([]policyv1alpha1.TargetReference, 0, len(refs))
	for _, ref := range refs {
		items = append(items, policyv1alpha1.TargetReference{Kind: ref.Kind, Name: ref.Name})
	}
	return items
}

func targetRefsToDTO(refs []policyv1alpha1.TargetReference) []dto.TargetRef {
	items := make([]dto.TargetRef, 0, len(refs))
	for _, ref := range refs {
		items = append(items, dto.TargetRef{Kind: ref.Kind, Name: ref.Name})
	}
	return items
}

func jwtAuthFromDTO(spec *dto.JWTAuthSpec) *policyv1alpha1.JWTAuthSpec {
	if spec == nil {
		return nil
	}
	return &policyv1alpha1.JWTAuthSpec{
		Issuer:      spec.Issuer,
		Audiences:   append([]string(nil), spec.Audiences...),
		FromHeaders: headerSourcesFromDTO(spec.FromHeaders),
	}
}

func jwtAuthToDTO(spec *policyv1alpha1.JWTAuthSpec) *dto.JWTAuthSpec {
	if spec == nil {
		return nil
	}
	return &dto.JWTAuthSpec{
		Issuer:      spec.Issuer,
		Audiences:   append([]string(nil), spec.Audiences...),
		FromHeaders: headerSourcesToDTO(spec.FromHeaders),
	}
}

func apiKeyAuthFromDTO(spec *dto.APIKeyAuthSpec) *policyv1alpha1.APIKeyAuthSpec {
	if spec == nil {
		return nil
	}
	return &policyv1alpha1.APIKeyAuthSpec{FromHeaders: headerSourcesFromDTO(spec.FromHeaders)}
}

func apiKeyAuthToDTO(spec *policyv1alpha1.APIKeyAuthSpec) *dto.APIKeyAuthSpec {
	if spec == nil {
		return nil
	}
	return &dto.APIKeyAuthSpec{FromHeaders: headerSourcesToDTO(spec.FromHeaders)}
}

func headerSourcesFromDTO(sources []dto.HeaderSource) []policyv1alpha1.HeaderSourceSpec {
	items := make([]policyv1alpha1.HeaderSourceSpec, 0, len(sources))
	for _, source := range sources {
		items = append(items, policyv1alpha1.HeaderSourceSpec{Name: source.Name, Prefix: source.Prefix})
	}
	return items
}

func headerSourcesToDTO(sources []policyv1alpha1.HeaderSourceSpec) []dto.HeaderSource {
	items := make([]dto.HeaderSource, 0, len(sources))
	for _, source := range sources {
		items = append(items, dto.HeaderSource{Name: source.Name, Prefix: source.Prefix})
	}
	return items
}

func timeoutFromDTO(spec *dto.TimeoutSpec) *policyv1alpha1.TimeoutSpec {
	if spec == nil {
		return nil
	}
	return &policyv1alpha1.TimeoutSpec{Duration: spec.Duration}
}

func timeoutToDTO(spec *policyv1alpha1.TimeoutSpec) *dto.TimeoutSpec {
	if spec == nil {
		return nil
	}
	return &dto.TimeoutSpec{Duration: spec.Duration}
}

func retryFromDTO(spec *dto.RetrySpec) *policyv1alpha1.RetrySpec {
	if spec == nil {
		return nil
	}
	return &policyv1alpha1.RetrySpec{Attempts: spec.Attempts, Conditions: append([]string(nil), spec.Conditions...)}
}

func retryToDTO(spec *policyv1alpha1.RetrySpec) *dto.RetrySpec {
	if spec == nil {
		return nil
	}
	return &dto.RetrySpec{Attempts: spec.Attempts, Conditions: append([]string(nil), spec.Conditions...)}
}

func rateLimitFromDTO(spec *dto.RateLimitSpec) *policyv1alpha1.RateLimitSpec {
	if spec == nil {
		return nil
	}
	return &policyv1alpha1.RateLimitSpec{
		RequestsPerUnit: spec.RequestsPerUnit,
		Unit:            spec.Unit,
		Scope:           spec.Scope,
	}
}

func rateLimitToDTO(spec *policyv1alpha1.RateLimitSpec) *dto.RateLimitSpec {
	if spec == nil {
		return nil
	}
	return &dto.RateLimitSpec{
		RequestsPerUnit: spec.RequestsPerUnit,
		Unit:            spec.Unit,
		Scope:           spec.Scope,
	}
}
