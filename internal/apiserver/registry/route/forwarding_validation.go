package route

import (
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
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
	if len(spec.UpstreamRefs) > apivalidation.MaxServiceTargets {
		errs = append(errs, field.TooMany(
			path.Child("upstreamRefs"),
			len(spec.UpstreamRefs),
			apivalidation.MaxServiceTargets,
		))
	}

	upstreamRefs := spec.UpstreamRefs
	if len(upstreamRefs) > apivalidation.MaxServiceTargets {
		upstreamRefs = upstreamRefs[:apivalidation.MaxServiceTargets]
	}
	seenUpstreamRefs := make(map[string]bool, len(upstreamRefs))
	for i, ref := range upstreamRefs {
		refPath := path.Child("upstreamRefs").Index(i)
		if ref.Name == "" {
			errs = append(errs, field.Required(refPath.Child("name"), "upstreamRef.name is required"))
		} else if !apivalidation.IsCanonicalID(ref.Name) {
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
		if ref.Weight < apivalidation.MinTargetWeight || ref.Weight > apivalidation.MaxTargetWeight {
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
	if len(spec.AI.Models) > apivalidation.MaxAIModels {
		errs = append(errs, field.TooMany(
			path.Child("ai", "models"),
			len(spec.AI.Models),
			apivalidation.MaxAIModels,
		))
	}

	models := spec.AI.Models
	if len(models) > apivalidation.MaxAIModels {
		models = models[:apivalidation.MaxAIModels]
	}
	modelsPath := path.Child("ai", "models")
	seenModels := make(map[string]bool, len(models))
	for i, model := range models {
		errs = append(errs, validateAIModel(model, modelsPath.Index(i), seenModels)...)
	}
	return errs
}

func validateAIModel(
	model resource.AIModel,
	path *field.Path,
	seenModels map[string]bool,
) field.ErrorList {
	var errs field.ErrorList
	if !apivalidation.IsValidModelName(model.Name) {
		errs = append(errs, field.Invalid(
			path.Child("name"),
			model.Name,
			"client model name is invalid",
		))
	} else if seenModels[model.Name] {
		errs = append(errs, field.Duplicate(path.Child("name"), model.Name))
	} else {
		seenModels[model.Name] = true
	}

	if len(model.Targets) == 0 {
		return append(errs, field.Required(path.Child("targets"), "at least one model target is required"))
	}
	if len(model.Targets) > apivalidation.MaxAIModelTargets {
		errs = append(errs, field.TooMany(
			path.Child("targets"),
			len(model.Targets),
			apivalidation.MaxAIModelTargets,
		))
	}
	targets := model.Targets
	if len(targets) > apivalidation.MaxAIModelTargets {
		targets = targets[:apivalidation.MaxAIModelTargets]
	}
	return append(errs, validateAIModelTargets(targets, path.Child("targets"))...)
}

func validateAIModelTargets(targets []resource.AIModelTarget, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	seenUpstreamRefs := make(map[string]bool, len(targets))
	for i, target := range targets {
		targetPath := path.Index(i)
		if target.UpstreamRef == "" {
			errs = append(errs, field.Required(targetPath.Child("upstreamRef"), "upstreamRef is required"))
		} else if !apivalidation.IsCanonicalID(target.UpstreamRef) {
			errs = append(errs, field.Invalid(
				targetPath.Child("upstreamRef"),
				target.UpstreamRef,
				"upstreamRef must be a canonical UUID",
			))
		} else if seenUpstreamRefs[target.UpstreamRef] {
			errs = append(errs, field.Duplicate(targetPath.Child("upstreamRef"), target.UpstreamRef))
		} else {
			seenUpstreamRefs[target.UpstreamRef] = true
		}
		if !apivalidation.IsValidModelName(target.Model) {
			errs = append(errs, field.Invalid(
				targetPath.Child("model"),
				target.Model,
				"upstream model name is invalid",
			))
		}
		if target.Weight < apivalidation.MinTargetWeight || target.Weight > apivalidation.MaxTargetWeight {
			errs = append(errs, field.Invalid(
				targetPath.Child("weight"),
				target.Weight,
				"weight is out of range",
			))
		}
	}
	return errs
}
