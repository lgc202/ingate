package route

import (
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func validateForwarding(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	if spec.AI != nil {
		return validateAIForwarding(spec, path)
	}

	if len(spec.UpstreamRefs) == 0 {
		return field.ErrorList{
			field.Required(path.Child("upstreamRefs"), "at least one upstreamRef is required"),
		}
	}
	var errs field.ErrorList
	if len(spec.UpstreamRefs) > routeconfig.MaxServiceTargets {
		errs = append(errs, field.TooMany(
			path.Child("upstreamRefs"),
			len(spec.UpstreamRefs),
			routeconfig.MaxServiceTargets,
		))
	}

	upstreamRefs := spec.UpstreamRefs
	if len(upstreamRefs) > routeconfig.MaxServiceTargets {
		upstreamRefs = upstreamRefs[:routeconfig.MaxServiceTargets]
	}
	seenUpstreamRefs := make(map[string]bool, len(upstreamRefs))
	for i, ref := range upstreamRefs {
		refPath := path.Child("upstreamRefs").Index(i)
		if ref.Name == "" {
			errs = append(errs, field.Required(refPath.Child("name"), "upstreamRef.name is required"))
		} else if !resourceconfig.IsCanonicalID(ref.Name) {
			errs = append(errs, field.Invalid(
				refPath.Child("name"),
				ref.Name,
				"upstreamRef.name must be a canonical UUID",
			))
		} else if seenUpstreamRefs[ref.Name] {
			errs = append(errs, field.Duplicate(refPath.Child("name"), ref.Name))
		} else {
			seenUpstreamRefs[ref.Name] = true
		}
		if ref.Weight < routeconfig.MinTargetWeight || ref.Weight > routeconfig.MaxTargetWeight {
			errs = append(errs, field.Invalid(
				refPath.Child("weight"),
				ref.Weight,
				"upstreamRef.weight is out of range",
			))
		}
	}
	return errs
}

func validateAIForwarding(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(spec.UpstreamRefs) != 0 {
		errs = append(errs, field.Forbidden(path.Child("upstreamRefs"), "AI route uses ai.models targets"))
	}
	if len(spec.Match.Methods) != 1 || !strings.EqualFold(spec.Match.Methods[0], http.MethodPost) {
		errs = append(errs, field.Invalid(
			path.Child("match", "methods"),
			spec.Match.Methods,
			"AI route currently requires POST",
		))
	}
	errs = append(errs, validateAIHeaders(spec, path)...)
	if len(spec.AI.Models) == 0 {
		return append(errs, field.Required(
			path.Child("ai", "models"),
			"at least one client model is required",
		))
	}
	if len(spec.AI.Models) > routeconfig.MaxAIModels {
		errs = append(errs, field.TooMany(
			path.Child("ai", "models"),
			len(spec.AI.Models),
			routeconfig.MaxAIModels,
		))
	}

	models := spec.AI.Models
	if len(models) > routeconfig.MaxAIModels {
		models = models[:routeconfig.MaxAIModels]
	}
	modelsPath := path.Child("ai", "models")
	seenModels := make(map[string]bool, len(models))
	for i, model := range models {
		modelPath := modelsPath.Index(i)
		if !routeconfig.IsValidModelName(model.Name) {
			errs = append(errs, field.Invalid(
				modelPath.Child("name"),
				model.Name,
				"client model name is invalid",
			))
		} else if seenModels[model.Name] {
			errs = append(errs, field.Duplicate(modelPath.Child("name"), model.Name))
		} else {
			seenModels[model.Name] = true
		}

		if len(model.Targets) == 0 {
			errs = append(errs, field.Required(modelPath.Child("targets"), "at least one model target is required"))
			continue
		}
		if len(model.Targets) > routeconfig.MaxAIModelTargets {
			errs = append(errs, field.TooMany(
				modelPath.Child("targets"),
				len(model.Targets),
				routeconfig.MaxAIModelTargets,
			))
		}
		targets := model.Targets
		if len(targets) > routeconfig.MaxAIModelTargets {
			targets = targets[:routeconfig.MaxAIModelTargets]
		}
		seenTargets := make(map[string]bool, len(targets))
		for j, target := range targets {
			targetPath := modelPath.Child("targets").Index(j)
			if target.UpstreamRef == "" {
				errs = append(errs, field.Required(targetPath.Child("upstreamRef"), "upstreamRef is required"))
			} else if !resourceconfig.IsCanonicalID(target.UpstreamRef) {
				errs = append(errs, field.Invalid(
					targetPath.Child("upstreamRef"),
					target.UpstreamRef,
					"upstreamRef must be a canonical UUID",
				))
			} else if seenTargets[target.UpstreamRef] {
				errs = append(errs, field.Duplicate(targetPath.Child("upstreamRef"), target.UpstreamRef))
			} else {
				seenTargets[target.UpstreamRef] = true
			}
			if !routeconfig.IsValidModelName(target.Model) {
				errs = append(errs, field.Invalid(
					targetPath.Child("model"),
					target.Model,
					"upstream model name is invalid",
				))
			}
			if target.Weight < routeconfig.MinTargetWeight || target.Weight > routeconfig.MaxTargetWeight {
				errs = append(errs, field.Invalid(
					targetPath.Child("weight"),
					target.Weight,
					"weight is out of range",
				))
			}
		}
	}
	return errs
}
