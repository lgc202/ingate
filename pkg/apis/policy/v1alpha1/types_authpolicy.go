package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthPolicySpec   `json:"spec,omitempty"`
	Status AuthPolicyStatus `json:"status,omitempty"`
}

type AuthPolicySpec struct {
	// +listType=atomic
	TargetRefs []TargetReference `json:"targetRefs,omitempty"`
	Type       string            `json:"type,omitempty"`
	JWT        *JWTAuthSpec      `json:"jwt,omitempty"`
	APIKey     *APIKeyAuthSpec   `json:"apiKey,omitempty"`
}

type TargetReference struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

type JWTAuthSpec struct {
	Issuer string `json:"issuer,omitempty"`
	// +listType=set
	Audiences []string `json:"audiences,omitempty"`
	// +listType=atomic
	FromHeaders []HeaderSourceSpec `json:"fromHeaders,omitempty"`
}

type APIKeyAuthSpec struct {
	// +listType=atomic
	FromHeaders []HeaderSourceSpec `json:"fromHeaders,omitempty"`
}

type HeaderSourceSpec struct {
	Name   string `json:"name,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type AuthPolicyStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthPolicy `json:"items"`
}
