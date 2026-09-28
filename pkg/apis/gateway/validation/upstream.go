package validation

import (
	"net/netip"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateUpstream 校验新建 Upstream 的资源身份和期望状态。
func ValidateUpstream(upstream *gateway.Upstream) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&upstream.ObjectMeta,
		false,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	return append(errs, validateUpstreamSpec(&upstream.Spec, field.NewPath("spec"))...)
}

// ValidateUpstreamUpdate 校验更新后的 Upstream 资源。
func ValidateUpstreamUpdate(next, previous *gateway.Upstream) field.ErrorList {
	errs := ValidateUpstream(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateUpstreamStatusUpdate 校验 Upstream Status 子资源更新。
func ValidateUpstreamStatusUpdate(next, previous *gateway.Upstream) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)
	return append(errs, validateConditions(next.Status.Conditions, next.Generation)...)
}

func validateUpstreamSpec(spec *gateway.UpstreamSpec, path *field.Path) field.ErrorList {
	endpointsPath := path.Child("endpoints")
	if len(spec.Endpoints) == 0 {
		return field.ErrorList{field.Required(endpointsPath, "must contain at least one endpoint")}
	}

	type endpointKey struct {
		address string
		port    int32
	}
	seen := make(map[endpointKey]bool, len(spec.Endpoints))

	var errs field.ErrorList
	for i, endpoint := range spec.Endpoints {
		endpointPath := endpointsPath.Index(i)
		key := endpointKey{address: endpoint.Address, port: endpoint.Port}
		if seen[key] {
			errs = append(errs, field.Duplicate(endpointPath, key))
		}
		seen[key] = true

		if _, err := netip.ParseAddr(endpoint.Address); err != nil {
			if messages := validation.IsDNS1123Subdomain(endpoint.Address); len(messages) > 0 {
				errs = append(errs, field.Invalid(
					endpointPath.Child("address"), endpoint.Address, strings.Join(messages, "; "),
				))
			}
		}

		if endpoint.Port < minPort || endpoint.Port > maxPort {
			errs = append(errs, field.Invalid(
				endpointPath.Child("port"), endpoint.Port, "must be between 1 and 65535",
			))
		}

		if endpoint.Weight <= 0 {
			errs = append(errs, field.Invalid(
				endpointPath.Child("weight"), endpoint.Weight, "must be greater than zero",
			))
		}
	}

	return errs
}
