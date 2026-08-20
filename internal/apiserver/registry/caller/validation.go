package caller

import (
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func validateCaller(caller *resource.Caller) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if caller.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}

	seenRoutes := make(map[string]struct{}, len(caller.Spec.RouteRefs))
	for i, routeID := range caller.Spec.RouteRefs {
		path := specPath.Child("routeRefs").Index(i)
		if routeID == "" {
			errs = append(errs, field.Required(path, "routeRef is required"))
		} else if _, err := uuid.Parse(routeID); err != nil {
			errs = append(errs, field.Invalid(path, routeID, "routeRef must be a UUID"))
		} else if _, exists := seenRoutes[routeID]; exists {
			errs = append(errs, field.Duplicate(path, routeID))
		}
		seenRoutes[routeID] = struct{}{}
	}

	seenKeys := make(map[string]struct{}, len(caller.Spec.AccessKeys))
	for i, key := range caller.Spec.AccessKeys {
		path := specPath.Child("accessKeys").Index(i)
		if _, err := uuid.Parse(key.ID); err != nil {
			errs = append(errs, field.Invalid(path.Child("id"), key.ID, "access key ID must be a UUID"))
		} else if _, exists := seenKeys[key.ID]; exists {
			errs = append(errs, field.Duplicate(path.Child("id"), key.ID))
		}
		seenKeys[key.ID] = struct{}{}
		if strings.TrimSpace(key.DisplayName) == "" {
			errs = append(errs, field.Required(path.Child("displayName"), "displayName is required"))
		}
		digest, err := hex.DecodeString(key.SecretDigest)
		if err != nil || len(digest) != 32 {
			errs = append(errs, field.Invalid(path.Child("secretDigest"), "<redacted>", "secretDigest must be a SHA-256 digest"))
		}
		if key.CreatedAt.IsZero() {
			errs = append(errs, field.Required(path.Child("createdAt"), "createdAt is required"))
		}
		if key.ExpiresAt != nil && !key.ExpiresAt.After(key.CreatedAt.Time) {
			errs = append(errs, field.Invalid(path.Child("expiresAt"), key.ExpiresAt, "expiresAt must be after createdAt"))
		}
	}
	return errs
}
