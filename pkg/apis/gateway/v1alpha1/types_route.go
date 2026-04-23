package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

type RouteSpec struct {
	// +listType=map
	// +listMapKey=name
	ParentRefs []ParentReference `json:"parentRefs,omitempty"`
	// +listType=set
	Hostnames []string `json:"hostnames,omitempty"`
	// +listType=atomic
	Rules []RouteRule `json:"rules,omitempty"`
}

type ParentReference struct {
	Name string `json:"name,omitempty"`
}

type RouteRule struct {
	// +listType=atomic
	Matches []HTTPRouteMatch `json:"matches,omitempty"`
	// +listType=atomic
	BackendRefs []BackendRef `json:"backendRefs,omitempty"`
	// +listType=atomic
	Filters []HTTPRouteFilter `json:"filters,omitempty"`
}

type HTTPRouteMatch struct {
	Path   *HTTPPathMatch `json:"path,omitempty"`
	Method string         `json:"method,omitempty"`
	// +listType=map
	// +listMapKey=name
	Headers []HTTPHeaderMatch `json:"headers,omitempty"`
}

type HTTPPathMatch struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type HTTPHeaderMatch struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type BackendRef struct {
	Name   string `json:"name,omitempty"`
	Port   int32  `json:"port,omitempty"`
	Weight int32  `json:"weight,omitempty"`
}

type HTTPRouteFilter struct {
	Type                   string                `json:"type,omitempty"`
	URLRewrite             *HTTPURLRewriteFilter `json:"urlRewrite,omitempty"`
	RequestHeaderModifier  *HTTPHeaderFilter     `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HTTPHeaderFilter     `json:"responseHeaderModifier,omitempty"`
}

type HTTPURLRewriteFilter struct {
	Path *HTTPPathModifier `json:"path,omitempty"`
}

type HTTPPathModifier struct {
	Type               string `json:"type,omitempty"`
	ReplacePrefixMatch string `json:"replacePrefixMatch,omitempty"`
}

type HTTPHeaderFilter struct {
	// +listType=map
	// +listMapKey=name
	Set []HTTPHeader `json:"set,omitempty"`
	// +listType=map
	// +listMapKey=name
	Add []HTTPHeader `json:"add,omitempty"`
	// +listType=set
	Remove []string `json:"remove,omitempty"`
}

type HTTPHeader struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type RouteStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +listType=map
	// +listMapKey=name
	Parents []RouteParentStatus `json:"parents,omitempty"`
}

type RouteParentStatus struct {
	Name string `json:"name,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Route `json:"items"`
}
