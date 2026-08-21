package caller

import (
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateCallerRequest) (resource.CallerSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.CallerSpec{}, adminservice.BadRequest("调用方名称不能为空")
	}
	routes, err := routeRefs(request.GetRouteIds())
	if err != nil {
		return resource.CallerSpec{}, err
	}
	return resource.CallerSpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		RouteRefs:   routes,
	}, nil
}

func updateSpec(request *adminv1.UpdateCallerRequest) (resource.CallerSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.CallerSpec{}, adminservice.BadRequest("调用方名称不能为空")
	}
	routes, err := routeRefs(request.GetRouteIds())
	if err != nil {
		return resource.CallerSpec{}, err
	}
	return resource.CallerSpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		RouteRefs:   routes,
	}, nil
}

func routeRefs(routeIDs []string) ([]string, error) {
	refs := make([]string, 0, len(routeIDs))
	seen := make(map[string]struct{}, len(routeIDs))
	for _, routeID := range routeIDs {
		if _, exists := seen[routeID]; exists {
			return nil, adminservice.BadRequest("同一个路由只能授权一次")
		}
		seen[routeID] = struct{}{}
		refs = append(refs, routeID)
	}
	return refs, nil
}

func accessKey(name string, expiresAt *timestamppb.Timestamp) (string, *time.Time, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, adminservice.BadRequest("密钥名称不能为空")
	}
	expiration, err := optionalTime(expiresAt)
	if err != nil {
		return "", nil, err
	}
	return name, expiration, nil
}

func optionalTime(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, adminservice.BadRequest("到期时间格式不正确")
	}
	expiresAt := value.AsTime()
	return &expiresAt, nil
}
