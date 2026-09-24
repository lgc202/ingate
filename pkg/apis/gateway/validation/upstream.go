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

func validateUpstreamSpec(spec *gateway.UpstreamSpec, path *field.Path) field.ErrorList {
	endpointsPath := path.Child("endpoints")
	if len(spec.Endpoints) == 0 {
		return field.ErrorList{field.Required(endpointsPath, "must contain at least one endpoint")}
	}

	var errs field.ErrorList
	for i, endpoint := range spec.Endpoints {
		endpointPath := endpointsPath.Index(i)
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
