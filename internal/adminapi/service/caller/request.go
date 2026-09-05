package caller

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/callerconfig"
)

func parseCallerSpec(
	displayName string,
	enabled bool,
	routeIDs []string,
) (resource.CallerSpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.CallerSpec{}, adminv1.ErrorInvalidArgument("调用方名称不能为空")
	}
	routeRefs, err := parseRouteIDs(routeIDs)
	if err != nil {
		return resource.CallerSpec{}, err
	}
	return resource.CallerSpec{
		DisplayName: displayName,
		Enabled:     enabled,
		RouteRefs:   routeRefs,
	}, nil
}

func parseRouteIDs(routeIDs []string) ([]string, error) {
	if len(routeIDs) > callerconfig.MaxRouteRefs {
		return nil, adminv1.ErrorInvalidArgument("授权路由数量超过限制")
	}

	routeRefs := make([]string, 0, len(routeIDs))
	seenRouteIDs := make(map[string]bool, len(routeIDs))
	for _, routeID := range routeIDs {
		parsedRouteID, err := uuid.Parse(routeID)
		if err != nil || parsedRouteID.String() != routeID {
			return nil, adminv1.ErrorInvalidArgument("授权路由 ID 格式不正确")
		}
		if seenRouteIDs[routeID] {
			return nil, adminv1.ErrorInvalidArgument("同一个路由只能授权一次")
		}
		seenRouteIDs[routeID] = true
		routeRefs = append(routeRefs, routeID)
	}
	slices.Sort(routeRefs)
	return routeRefs, nil
}

func parseAccessKey(
	displayName string,
	expiresAt *timestamppb.Timestamp,
) (string, *time.Time, error) {
	displayName = strings.TrimSpace(displayName)
	if !callerconfig.IsValidAccessKeyDisplayName(displayName) {
		return "", nil, adminv1.ErrorInvalidArgument("密钥名称不能为空或超过长度限制")
	}
	expiration, err := parseExpiration(expiresAt)
	if err != nil {
		return "", nil, err
	}
	return displayName, expiration, nil
}

func parseExpiration(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, adminv1.ErrorInvalidArgument("到期时间格式不正确").WithCause(err)
	}
	return new(value.AsTime()), nil
}
