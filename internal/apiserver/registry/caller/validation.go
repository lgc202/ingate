package caller

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	"github.com/lgc202/ingate/internal/pkg/accesskey"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/callerconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

func validateCaller(caller *resource.Caller) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(caller.Name, field.NewPath("metadata", "name"))
	errs = append(errs, apiregistry.ValidateDisplayName(
		caller.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	errs = append(errs, validateRouteRefs(
		caller.Spec.RouteRefs,
		specPath.Child("routeRefs"),
	)...)
	errs = append(errs, validateAccessKeys(
		caller.Spec.AccessKeys,
		specPath.Child("accessKeys"),
	)...)
	return errs
}

func validateRouteRefs(routeIDs []string, path *field.Path) field.ErrorList {
	routeCount := len(routeIDs)
	var errs field.ErrorList
	if routeCount > callerconfig.MaxRouteRefs {
		errs = append(errs, field.TooMany(path, routeCount, callerconfig.MaxRouteRefs))
		routeIDs = routeIDs[:callerconfig.MaxRouteRefs]
	}

	seenRouteIDs := make(map[string]bool, len(routeIDs))
	for i, routeID := range routeIDs {
		routePath := path.Index(i)
		if routeID == "" {
			errs = append(errs, field.Required(routePath, "routeRef is required"))
		} else if !resourceconfig.IsCanonicalID(routeID) {
			errs = append(errs, field.Invalid(routePath, routeID, "routeRef must be a canonical UUID"))
		} else if seenRouteIDs[routeID] {
			errs = append(errs, field.Duplicate(routePath, routeID))
		}
		seenRouteIDs[routeID] = true
	}
	return errs
}

func validateAccessKeys(accessKeys []resource.AccessKey, path *field.Path) field.ErrorList {
	accessKeyCount := len(accessKeys)
	var errs field.ErrorList
	if accessKeyCount > callerconfig.MaxAccessKeys {
		errs = append(errs, field.TooMany(path, accessKeyCount, callerconfig.MaxAccessKeys))
		accessKeys = accessKeys[:callerconfig.MaxAccessKeys]
	}

	seenAccessKeyIDs := make(map[string]bool, len(accessKeys))
	seenDisplayNames := make([]string, 0, len(accessKeys))
	for i, accessKey := range accessKeys {
		accessKeyPath := path.Index(i)
		if !resourceconfig.IsCanonicalID(accessKey.ID) {
			errs = append(errs, field.Invalid(
				accessKeyPath.Child("id"),
				accessKey.ID,
				"access key ID must be a canonical UUID",
			))
		} else if seenAccessKeyIDs[accessKey.ID] {
			errs = append(errs, field.Duplicate(accessKeyPath.Child("id"), accessKey.ID))
		}
		seenAccessKeyIDs[accessKey.ID] = true

		if !callerconfig.IsValidAccessKeyDisplayName(accessKey.DisplayName) {
			errs = append(errs, field.Invalid(
				accessKeyPath.Child("displayName"),
				accessKey.DisplayName,
				fmt.Sprintf(
					"displayName is required and must not exceed %d bytes",
					callerconfig.MaxAccessKeyDisplayNameBytes,
				),
			))
		} else {
			duplicateDisplayName := slices.ContainsFunc(
				seenDisplayNames,
				func(candidate string) bool {
					return strings.EqualFold(candidate, accessKey.DisplayName)
				},
			)
			if duplicateDisplayName {
				errs = append(errs, field.Duplicate(
					accessKeyPath.Child("displayName"),
					accessKey.DisplayName,
				))
			}
			seenDisplayNames = append(seenDisplayNames, accessKey.DisplayName)
		}

		if !accesskey.IsValidDigest(accessKey.SecretDigest) {
			errs = append(errs, field.Invalid(
				accessKeyPath.Child("secretDigest"),
				"<redacted>",
				"secretDigest must be a SHA-256 digest",
			))
		}
		if accessKey.CreatedAt.IsZero() {
			errs = append(errs, field.Required(
				accessKeyPath.Child("createdAt"),
				"createdAt is required",
			))
		}
		if accessKey.ExpiresAt != nil && !accessKey.ExpiresAt.After(accessKey.CreatedAt.Time) {
			errs = append(errs, field.Invalid(
				accessKeyPath.Child("expiresAt"),
				accessKey.ExpiresAt,
				"expiresAt must be after createdAt",
			))
		}
	}
	return errs
}
