package caller

import (
	"slices"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func callerResponse(caller *resource.Caller) *adminv1.Caller {
	accessKeys := make([]*adminv1.AccessKey, len(caller.Spec.AccessKeys))
	for i, accessKey := range caller.Spec.AccessKeys {
		accessKeys[i] = accessKeyResponse(accessKey)
	}
	return &adminv1.Caller{
		Id:         caller.Name,
		Name:       caller.Spec.DisplayName,
		Enabled:    caller.Spec.Enabled,
		RouteIds:   slices.Clone(caller.Spec.RouteRefs),
		AccessKeys: accessKeys,
		Version:    caller.Generation,
		CreatedAt:  adminservice.Timestamp(caller.CreationTimestamp.Time),
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
