// Package gateway 实现 Gateway 管理 API
package gateway

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	standaloneHTTPPort  = 8080
	standaloneHTTPSPort = 8443
)

// ProviderSet 提供 Gateway 协议服务
var ProviderSet = wire.NewSet(NewService)

// Service 实现网关入口管理 API
type Service struct {
	usecase *gatewaybiz.Usecase
}

// NewService 创建网关入口协议服务
func NewService(usecase *gatewaybiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListGateways(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListGatewaysReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, adminservice.OperationError(err, "查询网关失败")
	}
	reply := &adminv1.ListGatewaysReply{Gateways: make([]*adminv1.Gateway, 0, len(items))}
	for i := range items {
		reply.Gateways = append(reply.Gateways, gatewayReply(&items[i]))
	}
	return reply, nil
}

func (s *Service) GetGateway(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.GetGatewayReply, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, adminservice.OperationError(err, "查询网关失败")
	}
	return &adminv1.GetGatewayReply{Gateway: gatewayReply(item)}, nil
}

func (s *Service) CreateGateway(ctx context.Context, request *adminv1.CreateGatewayRequest) (*adminv1.MutationReply, error) {
	spec, err := gatewaySpec(request.GetName(), request.GetDescription(), request.GetListeners(), request.GetHostnames())
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, adminservice.OperationError(err, "创建网关失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateGateway(ctx context.Context, request *adminv1.UpdateGatewayRequest) (*adminv1.MutationReply, error) {
	spec, err := gatewaySpec(request.GetName(), request.GetDescription(), request.GetListeners(), request.GetHostnames())
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, adminservice.OperationError(err, "更新网关失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetGatewayEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, adminservice.OperationError(err, "更新网关状态失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteGateway(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, adminservice.OperationError(err, "删除网关失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func gatewaySpec(
	name string,
	description string,
	inputListeners []*adminv1.GatewayListener,
	inputHostnames []string,
) (resource.GatewaySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.GatewaySpec{}, adminservice.BadRequest("网关名称不能为空")
	}
	if len(inputListeners) == 0 {
		return resource.GatewaySpec{}, adminservice.BadRequest("至少需要启用一个运行入口")
	}

	listeners := make([]resource.Listener, 0, len(inputListeners))
	listenerRefs := make([]string, 0, len(inputListeners))
	seenProtocols := make(map[resource.Protocol]struct{}, len(inputListeners))
	for _, input := range inputListeners {
		if input == nil {
			return resource.GatewaySpec{}, adminservice.BadRequest("运行入口配置不能为空")
		}
		protocol := resource.Protocol(input.GetProtocol())
		if _, exists := seenProtocols[protocol]; exists {
			return resource.GatewaySpec{}, adminservice.BadRequest("同一种运行入口协议只能配置一次")
		}
		seenProtocols[protocol] = struct{}{}
		certificateID := strings.TrimSpace(input.GetCertificateId())
		switch protocol {
		case resource.ProtocolHTTP:
			if input.GetPort() != standaloneHTTPPort {
				return resource.GatewaySpec{}, adminservice.BadRequest("HTTP 运行入口端口必须为 8080")
			}
			if certificateID != "" {
				return resource.GatewaySpec{}, adminservice.BadRequest("HTTP 运行入口不能配置证书")
			}
		case resource.ProtocolHTTPS:
			if input.GetPort() != standaloneHTTPSPort {
				return resource.GatewaySpec{}, adminservice.BadRequest("HTTPS 运行入口端口必须为 8443")
			}
			if certificateID == "" {
				return resource.GatewaySpec{}, adminservice.BadRequest("HTTPS 运行入口必须选择证书")
			}
		default:
			return resource.GatewaySpec{}, adminservice.BadRequest("运行入口协议不正确")
		}
		listenerName := strings.ToLower(string(protocol))
		listeners = append(listeners, resource.Listener{
			Name: listenerName, Protocol: protocol, Port: int(input.GetPort()), CertificateRef: certificateID,
		})
		listenerRefs = append(listenerRefs, listenerName)
	}
	slices.SortFunc(listeners, func(a, b resource.Listener) int {
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})

	hostnames := make([]string, 0, len(inputHostnames))
	for _, value := range inputHostnames {
		hostname, ok := hostnameutil.Normalize(strings.ToLower(strings.TrimSpace(value)))
		if !ok || hostname == "*" {
			return resource.GatewaySpec{}, adminservice.BadRequest("网关域名格式不正确")
		}
		if slices.Contains(hostnames, hostname) {
			continue
		}
		for _, existing := range hostnames {
			if hostnameutil.Overlaps(hostname, existing) {
				return resource.GatewaySpec{}, adminservice.BadRequest(fmt.Sprintf("网关域名 %q 与 %q 的范围重叠", hostname, existing))
			}
		}
		hostnames = append(hostnames, hostname)
	}

	bindings := make([]resource.HostBinding, 0, len(hostnames))
	for _, hostname := range hostnames {
		bindings = append(bindings, resource.HostBinding{
			Hostname: hostname, ListenerRefs: append([]string(nil), listenerRefs...),
		})
	}
	return resource.GatewaySpec{
		DisplayName: name, Description: description, Listeners: listeners, HostBindings: bindings,
	}, nil
}

func gatewayReply(gateway *resource.Gateway) *adminv1.Gateway {
	status := biz.EnabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions)
	listeners := make([]*adminv1.GatewayListener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		listeners = append(listeners, &adminv1.GatewayListener{
			Protocol: string(listener.Protocol), Port: int32(listener.Port), CertificateId: listener.CertificateRef,
		})
	}
	hostnames := make([]string, 0, len(gateway.Spec.HostBindings))
	for _, binding := range gateway.Spec.HostBindings {
		if binding.Hostname != "" {
			hostnames = append(hostnames, binding.Hostname)
		}
	}
	return &adminv1.Gateway{
		Id:          gateway.Name,
		Version:     strconv.FormatInt(gateway.Generation, 10),
		Status:      adminservice.ResourceStatus(status),
		Name:        gateway.Spec.DisplayName,
		Description: gateway.Spec.Description,
		Listeners:   listeners,
		Hostnames:   hostnames,
		Enabled:     gateway.Spec.Enabled,
		CreatedAt:   adminservice.Timestamp(gateway.CreationTimestamp.Time),
	}
}
