package route

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func validateAIHeaders(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	for i, header := range spec.Match.Headers {
		if aiprotocol.IsInternalHeader(header.Name) {
			errs = append(errs, field.Forbidden(
				path.Child("match", "headers").Index(i).Child("name"),
				"header is reserved by AI routing",
			))
		}
	}
	errs = append(errs, validateAIHeaderModifier(
		spec.RequestHeaderModifier,
		path.Child("requestHeaderModifier"),
	)...)
	return errs
}

func validateAIHeaderModifier(
	modifier *resource.HeaderModifier,
	path *field.Path,
) field.ErrorList {
	if modifier == nil {
		return nil
	}

	var errs field.ErrorList
	for i, header := range modifier.Set {
		if aiprotocol.IsInternalHeader(header.Name) {
			errs = append(errs, field.Forbidden(
				path.Child("set").Index(i).Child("name"),
				"header is reserved by AI routing",
			))
		}
	}
	for i, header := range modifier.Add {
		if aiprotocol.IsInternalHeader(header.Name) {
			errs = append(errs, field.Forbidden(
				path.Child("add").Index(i).Child("name"),
				"header is reserved by AI routing",
			))
		}
	}
	for i, name := range modifier.Remove {
		if aiprotocol.IsInternalHeader(name) {
			errs = append(errs, field.Forbidden(
				path.Child("remove").Index(i),
				"header is reserved by AI routing",
			))
		}
	}
	return errs
}

func validateHeaderModifier(modifier *resource.HeaderModifier, path *field.Path) field.ErrorList {
	if modifier == nil {
		return nil
	}
	actionCount := len(modifier.Set) + len(modifier.Add) + len(modifier.Remove)
	if actionCount == 0 {
		return field.ErrorList{field.Required(path, "at least one header modifier action is required")}
	}
	var errs field.ErrorList
	if actionCount > routeconfig.MaxHeaderModifierActions {
		errs = append(errs, field.TooMany(path, actionCount, routeconfig.MaxHeaderModifierActions))
	}
	usedNames := make(map[string]bool, actionCount)
	errs = append(errs, validateHeaderValues(modifier.Set, path.Child("set"), usedNames)...)
	errs = append(errs, validateHeaderValues(modifier.Add, path.Child("add"), usedNames)...)
	for i, name := range modifier.Remove {
		namePath := path.Child("remove").Index(i)
		if !httpheader.IsValidName(name) {
			errs = append(errs, field.Invalid(namePath, name, "header name is invalid"))
			continue
		}
		key := httpheader.NormalizeName(name)
		if usedNames[key] {
			errs = append(errs, field.Duplicate(namePath, name))
		} else {
			usedNames[key] = true
		}
	}
	return errs
}

func validateHeaderValues(
	values []resource.HeaderValue,
	path *field.Path,
	usedNames map[string]bool,
) field.ErrorList {
	var errs field.ErrorList
	for i, value := range values {
		valuePath := path.Index(i)
		validName := httpheader.IsValidName(value.Name)
		if !validName {
			errs = append(errs, field.Invalid(valuePath.Child("name"), value.Name, "header name is invalid"))
		}
		if value.Value == "" || !httpheader.IsValidValue(value.Value) {
			errs = append(errs, field.Invalid(valuePath.Child("value"), value.Value, "header value is invalid"))
		}
		if validName {
			key := httpheader.NormalizeName(value.Name)
			if usedNames[key] {
				errs = append(errs, field.Duplicate(valuePath.Child("name"), value.Name))
			} else {
				usedNames[key] = true
			}
		}
	}
	return errs
}
