package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TrafficPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrafficPolicySpec   `json:"spec,omitempty"`
	Status TrafficPolicyStatus `json:"status,omitempty"`
}

type TrafficPolicySpec struct {
	// +listType=atomic
	TargetRefs []TargetReference `json:"targetRefs,omitempty"`
	Timeout    *TimeoutSpec      `json:"timeout,omitempty"`
	Retry      *RetrySpec        `json:"retry,omitempty"`
	RateLimit  *RateLimitSpec    `json:"rateLimit,omitempty"`
}

type TimeoutSpec struct {
	Duration string `json:"duration,omitempty"`
}

type RetrySpec struct {
	Attempts int32 `json:"attempts,omitempty"`
	// +listType=set
	Conditions []string `json:"conditions,omitempty"`
}

type RateLimitSpec struct {
	RequestsPerUnit int32  `json:"requestsPerUnit,omitempty"`
	Unit            string `json:"unit,omitempty"`
	Scope           string `json:"scope,omitempty"`
}

type TrafficPolicyStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TrafficPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrafficPolicy `json:"items"`
}
