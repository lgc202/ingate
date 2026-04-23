package dto

type CreateRouteRequest struct {
	Name       string      `json:"name" binding:"required"`
	ParentRefs []ParentRef `json:"parentRefs" binding:"required,min=1,dive"`
	Hostnames  []string    `json:"hostnames,omitempty"`
	Rules      []RouteRule `json:"rules" binding:"required,min=1,dive"`
}

type UpdateRouteRequest struct {
	ParentRefs []ParentRef `json:"parentRefs" binding:"required,min=1,dive"`
	Hostnames  []string    `json:"hostnames,omitempty"`
	Rules      []RouteRule `json:"rules" binding:"required,min=1,dive"`
}

type ParentRef struct {
	Name string `json:"name" binding:"required"`
}

type RouteRule struct {
	Matches     []HTTPRouteMatch  `json:"matches" binding:"required,min=1,dive"`
	BackendRefs []BackendRef      `json:"backendRefs" binding:"required,min=1,dive"`
	Filters     []HTTPRouteFilter `json:"filters,omitempty"`
}

type HTTPRouteMatch struct {
	Path    *HTTPPathMatch    `json:"path,omitempty"`
	Method  string            `json:"method,omitempty"`
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
	Name   string `json:"name" binding:"required"`
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
	Set    []HTTPHeader `json:"set,omitempty"`
	Add    []HTTPHeader `json:"add,omitempty"`
	Remove []string     `json:"remove,omitempty"`
}

type HTTPHeader struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type RouteResponse struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     RouteSpec       `json:"spec"`
	Status   RouteStatusView `json:"status,omitempty"`
}

type RouteSpec struct {
	ParentRefs []ParentRef `json:"parentRefs,omitempty"`
	Hostnames  []string    `json:"hostnames,omitempty"`
	Rules      []RouteRule `json:"rules,omitempty"`
}

type RouteStatusView struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	Conditions         []Condition         `json:"conditions,omitempty"`
	Parents            []RouteParentStatus `json:"parents,omitempty"`
}

type RouteParentStatus struct {
	Name       string      `json:"name"`
	Conditions []Condition `json:"conditions,omitempty"`
}

type RouteListResponse struct {
	Items []RouteResponse `json:"items"`
}
