package route

import (
	"context"
	"net/http"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

const (
	minRouteTimeoutMillis  = 100
	maxRouteTimeoutMillis  = 300000
	minRetryAttempts       = 1
	maxRetryAttempts       = 5
	minPerTryTimeoutMillis = 100
	maxPerTryTimeoutMillis = 60000
)

// strategy 定义 Route 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 Route status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	route := obj.(*resource.Route)
	route.Status = resource.ResourceStatus{}
	route.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Status = oldRoute.Status
	newRoute.Generation = oldRoute.Generation
	if !apiequality.Semantic.DeepEqual(oldRoute.Spec, newRoute.Spec) {
		newRoute.Generation = oldRoute.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Spec = oldRoute.Spec
	metav1.ResetObjectMetaForStatus(&newRoute.ObjectMeta, &oldRoute.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateRoute(route *resource.Route) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if len(route.Spec.ParentRefs) == 0 {
		errs = append(errs, field.Required(specPath.Child("parentRefs"), "at least one parentRef is required"))
	}
	for i, parentRef := range route.Spec.ParentRefs {
		if parentRef.Name == "" {
			errs = append(errs, field.Required(specPath.Child("parentRefs").Index(i).Child("name"), "parentRef.name is required"))
		}
	}

	for i, hostname := range route.Spec.Hostnames {
		if hostname == "" {
			errs = append(errs, field.Required(specPath.Child("hostnames").Index(i), "hostname cannot be empty"))
		} else if !validHostname(hostname) {
			errs = append(errs, field.Invalid(specPath.Child("hostnames").Index(i), hostname, "hostname is invalid"))
		}
	}

	if len(route.Spec.Rules) == 0 {
		errs = append(errs, field.Required(specPath.Child("rules"), "at least one rule is required"))
		return errs
	}

	seenRuleNames := make(map[string]struct{}, len(route.Spec.Rules))
	for i, rule := range route.Spec.Rules {
		rulePath := specPath.Child("rules").Index(i)
		if rule.Name == "" {
			errs = append(errs, field.Required(rulePath.Child("name"), "name is required"))
		} else if _, ok := seenRuleNames[rule.Name]; ok {
			errs = append(errs, field.Duplicate(rulePath.Child("name"), rule.Name))
		} else {
			seenRuleNames[rule.Name] = struct{}{}
		}
		if rule.PathPrefix == "" {
			errs = append(errs, field.Required(rulePath.Child("pathPrefix"), "pathPrefix is required"))
		} else if !strings.HasPrefix(rule.PathPrefix, "/") {
			errs = append(errs, field.Invalid(rulePath.Child("pathPrefix"), rule.PathPrefix, "pathPrefix must start with /"))
		}
		for j, method := range rule.Methods {
			if !validHTTPMethod(method) {
				errs = append(errs, field.NotSupported(rulePath.Child("methods").Index(j), method, []string{"GET", "POST", "PUT", "PATCH", "DELETE"}))
			}
		}
		for j, filter := range rule.Filters {
			filterPath := rulePath.Child("filters").Index(j)
			switch filter.Type {
			case resource.RouteFilterRequestHeaderModifier:
				if filter.RequestHeaderModifier == nil {
					errs = append(errs, field.Required(filterPath.Child("requestHeaderModifier"), "requestHeaderModifier is required"))
				} else {
					errs = append(errs, validateHeaderModifier(filter.RequestHeaderModifier, filterPath.Child("requestHeaderModifier"))...)
				}
			case resource.RouteFilterResponseHeaderModifier:
				if filter.ResponseHeaderModifier == nil {
					errs = append(errs, field.Required(filterPath.Child("responseHeaderModifier"), "responseHeaderModifier is required"))
				} else {
					errs = append(errs, validateHeaderModifier(filter.ResponseHeaderModifier, filterPath.Child("responseHeaderModifier"))...)
				}
			default:
				errs = append(errs, field.NotSupported(filterPath.Child("type"), filter.Type, []string{
					string(resource.RouteFilterRequestHeaderModifier),
					string(resource.RouteFilterResponseHeaderModifier),
				}))
			}
		}
		if rule.Timeout != nil {
			if rule.Timeout.RequestMillis < minRouteTimeoutMillis || rule.Timeout.RequestMillis > maxRouteTimeoutMillis {
				errs = append(errs, field.Invalid(rulePath.Child("timeout").Child("requestMillis"), rule.Timeout.RequestMillis, "timeout.requestMillis is out of range"))
			}
		}
		if rule.Retry != nil {
			if rule.Retry.Attempts < minRetryAttempts || rule.Retry.Attempts > maxRetryAttempts {
				errs = append(errs, field.Invalid(rulePath.Child("retry").Child("attempts"), rule.Retry.Attempts, "retry.attempts is out of range"))
			}
			if rule.Retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || rule.Retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis {
				errs = append(errs, field.Invalid(rulePath.Child("retry").Child("perTryTimeoutMillis"), rule.Retry.PerTryTimeoutMillis, "retry.perTryTimeoutMillis is out of range"))
			}
			if rule.Timeout != nil && rule.Retry.PerTryTimeoutMillis > rule.Timeout.RequestMillis {
				errs = append(errs, field.Invalid(rulePath.Child("retry").Child("perTryTimeoutMillis"), rule.Retry.PerTryTimeoutMillis, "retry.perTryTimeoutMillis must be less than or equal to timeout.requestMillis"))
			}
		}
		if len(rule.UpstreamRefs) == 0 {
			errs = append(errs, field.Required(rulePath.Child("upstreamRefs"), "at least one upstreamRef is required"))
		}
		for j, upstreamRef := range rule.UpstreamRefs {
			upstreamPath := rulePath.Child("upstreamRefs").Index(j)
			if upstreamRef.Name == "" {
				errs = append(errs, field.Required(upstreamPath.Child("name"), "upstreamRef.name is required"))
			}
			if upstreamRef.Weight < 1 || upstreamRef.Weight > 1000 {
				errs = append(errs, field.Invalid(upstreamPath.Child("weight"), upstreamRef.Weight, "upstreamRef.weight must be between 1 and 1000"))
			}
		}
	}
	return errs
}

func validateHeaderModifier(modifier *resource.HeaderModifier, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if len(modifier.Set) == 0 && len(modifier.Add) == 0 && len(modifier.Remove) == 0 {
		return append(errs, field.Required(path, "at least one header modifier action is required"))
	}
	errs = append(errs, validateHeaderValues(modifier.Set, path.Child("set"))...)
	errs = append(errs, validateHeaderValues(modifier.Add, path.Child("add"))...)
	for i, name := range modifier.Remove {
		if name == "" {
			errs = append(errs, field.Required(path.Child("remove").Index(i), "header name is required"))
		}
	}
	return errs
}

func validateHeaderValues(values []resource.HeaderValue, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	for i, value := range values {
		valuePath := path.Index(i)
		if value.Name == "" {
			errs = append(errs, field.Required(valuePath.Child("name"), "header name is required"))
		}
		if value.Value == "" {
			errs = append(errs, field.Required(valuePath.Child("value"), "header value is required"))
		}
	}
	return errs
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validHostname(hostname string) bool {
	if len(hostname) > 2 && hostname[:2] == "*." {
		hostname = hostname[2:]
	}
	hostname = strings.ToLower(hostname)
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	for i, r := range label {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid {
			return false
		}
		if (i == 0 || i == len(label)-1) && r == '-' {
			return false
		}
	}
	return true
}
