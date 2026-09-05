package caller

import (
	"slices"

	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func callerResponse(caller *resource.Caller) *adminv1.Caller {
	return &adminv1.Caller{
		Id:       caller.Name,
		Name:     caller.Spec.DisplayName,
		Enabled:  caller.Spec.Enabled,
		RouteIds: slices.Clone(caller.Spec.RouteRefs),
		AccessKeys: lo.Map(caller.Spec.AccessKeys, func(key resource.AccessKey, _ int) *adminv1.AccessKey {
			return accessKeyResponse(key)
		}),
		Version:   caller.Generation,
		CreatedAt: adminservice.Timestamp(caller.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(
			adminservice.ResourceUpdatedAt(caller.Annotations),
		),
	}
}

func accessKeyResponse(key resource.AccessKey) *adminv1.AccessKey {
	response := &adminv1.AccessKey{
		Id:        key.ID,
		Name:      key.DisplayName,
		Enabled:   key.Enabled,
		CreatedAt: adminservice.Timestamp(key.CreatedAt.Time),
	}
	if key.ExpiresAt != nil {
		response.ExpiresAt = adminservice.Timestamp(key.ExpiresAt.Time)
	}
	return response
}

func issuedAccessKeyResponse(issued callerbiz.IssuedAccessKey) *adminv1.IssuedAccessKey {
	return &adminv1.IssuedAccessKey{
		AccessKey: accessKeyResponse(issued.AccessKey),
		Secret:    issued.Secret,
	}
}
