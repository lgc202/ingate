package dto

type CreateAuthPolicyRequest struct {
	Name       string          `json:"name" binding:"required"`
	TargetRefs []TargetRef     `json:"targetRefs" binding:"required,min=1,dive"`
	Type       string          `json:"type" binding:"required"`
	JWT        *JWTAuthSpec    `json:"jwt,omitempty"`
	APIKey     *APIKeyAuthSpec `json:"apiKey,omitempty"`
}

type UpdateAuthPolicyRequest struct {
	TargetRefs []TargetRef     `json:"targetRefs" binding:"required,min=1,dive"`
	Type       string          `json:"type" binding:"required"`
	JWT        *JWTAuthSpec    `json:"jwt,omitempty"`
	APIKey     *APIKeyAuthSpec `json:"apiKey,omitempty"`
}

type TargetRef struct {
	Kind string `json:"kind" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type JWTAuthSpec struct {
	Issuer      string         `json:"issuer,omitempty"`
	Audiences   []string       `json:"audiences,omitempty"`
	FromHeaders []HeaderSource `json:"fromHeaders,omitempty"`
}

type APIKeyAuthSpec struct {
	FromHeaders []HeaderSource `json:"fromHeaders,omitempty"`
}

type HeaderSource struct {
	Name   string `json:"name,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type AuthPolicyResponse struct {
	Metadata ObjectMeta           `json:"metadata"`
	Spec     AuthPolicySpec       `json:"spec"`
	Status   AuthPolicyStatusView `json:"status,omitempty"`
}

type AuthPolicySpec struct {
	TargetRefs []TargetRef     `json:"targetRefs,omitempty"`
	Type       string          `json:"type,omitempty"`
	JWT        *JWTAuthSpec    `json:"jwt,omitempty"`
	APIKey     *APIKeyAuthSpec `json:"apiKey,omitempty"`
}

type AuthPolicyStatusView struct {
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

type AuthPolicyListResponse struct {
	Items []AuthPolicyResponse `json:"items"`
}
