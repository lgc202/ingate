package dto

type CreateTrafficPolicyRequest struct {
	Name       string         `json:"name" binding:"required"`
	TargetRefs []TargetRef    `json:"targetRefs" binding:"required,min=1,dive"`
	Timeout    *TimeoutSpec   `json:"timeout,omitempty"`
	Retry      *RetrySpec     `json:"retry,omitempty"`
	RateLimit  *RateLimitSpec `json:"rateLimit,omitempty"`
}

type UpdateTrafficPolicyRequest struct {
	TargetRefs []TargetRef    `json:"targetRefs" binding:"required,min=1,dive"`
	Timeout    *TimeoutSpec   `json:"timeout,omitempty"`
	Retry      *RetrySpec     `json:"retry,omitempty"`
	RateLimit  *RateLimitSpec `json:"rateLimit,omitempty"`
}

type TimeoutSpec struct {
	Duration string `json:"duration,omitempty"`
}

type RetrySpec struct {
	Attempts   int32    `json:"attempts,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
}

type RateLimitSpec struct {
	RequestsPerUnit int32  `json:"requestsPerUnit,omitempty"`
	Unit            string `json:"unit,omitempty"`
	Scope           string `json:"scope,omitempty"`
}

type TrafficPolicyResponse struct {
	Metadata ObjectMeta              `json:"metadata"`
	Spec     TrafficPolicySpec       `json:"spec"`
	Status   TrafficPolicyStatusView `json:"status,omitempty"`
}

type TrafficPolicySpec struct {
	TargetRefs []TargetRef    `json:"targetRefs,omitempty"`
	Timeout    *TimeoutSpec   `json:"timeout,omitempty"`
	Retry      *RetrySpec     `json:"retry,omitempty"`
	RateLimit  *RateLimitSpec `json:"rateLimit,omitempty"`
}

type TrafficPolicyStatusView struct {
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

type TrafficPolicyListResponse struct {
	Items []TrafficPolicyResponse `json:"items"`
}
