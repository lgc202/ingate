package caller

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func callerResponse(caller *resource.Caller) *adminv1.Caller {
	response := &adminv1.Caller{
		Id:         caller.Name,
		Name:       caller.Spec.DisplayName,
		Enabled:    caller.Spec.Enabled,
		RouteIds:   append([]string(nil), caller.Spec.RouteRefs...),
		Version:    caller.Generation,
		CreatedAt:  adminservice.Timestamp(caller.CreationTimestamp.Time),
		UpdatedAt:  adminservice.Timestamp(adminservice.ResourceUpdatedAt(caller.Annotations)),
		AccessKeys: make([]*adminv1.AccessKey, 0, len(caller.Spec.AccessKeys)),
	}
	for _, key := range caller.Spec.AccessKeys {
		response.AccessKeys = append(response.AccessKeys, accessKeyResponse(key))
	}
	return response
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

func issuedKeyResponse(issued callerbiz.IssuedKey) *adminv1.IssuedAccessKey {
	return &adminv1.IssuedAccessKey{
		AccessKey: accessKeyResponse(issued.AccessKey),
		Secret:    issued.Secret,
	}
}
