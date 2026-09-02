// Package servicemanagement 提供 Service 管理 API。
package servicemanagement

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	servicebiz "github.com/lgc202/ingate/internal/adminapi/biz/service"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现服务管理 API。
type Service struct {
	services *servicebiz.Usecase
}

// NewService 创建服务协议层。
func NewService(services *servicebiz.Usecase) *Service {
	return &Service{services: services}
}

// ListServices 返回满足筛选条件的服务列表。
func (s *Service) ListServices(
	ctx context.Context,
	request *adminv1.ListServicesRequest,
) (*adminv1.ListServicesResponse, error) {
	typeFilter := servicebiz.TypeAny
	switch request.GetType() {
	case adminv1.ServiceType_SERVICE_TYPE_UNSPECIFIED:
	case adminv1.ServiceType_SERVICE_TYPE_HTTP:
		typeFilter = servicebiz.TypeHTTP
	case adminv1.ServiceType_SERVICE_TYPE_MODEL:
		typeFilter = servicebiz.TypeModel
	default:
		return nil, adminv1.ErrorInvalidArgument("服务类型不正确")
	}

	filter := servicebiz.ListFilter{
		Filter: adminservice.ResourceFilter(
			request.GetQuery(),
			nil,
			request.GetState(),
		),
		Type: typeFilter,
	}
	page, err := s.services.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		filter,
	)
	if err != nil {
		return nil, err
	}
	services := make([]*adminv1.Service, len(page.Items))
	for i := range page.Items {
		services[i] = serviceResponse(&page.Items[i])
	}
	return &adminv1.ListServicesResponse{
		Services:   services,
		NextCursor: page.NextCursor,
	}, nil
}

// GetService 返回指定服务。
func (s *Service) GetService(
	ctx context.Context,
	request *adminv1.GetServiceRequest,
) (*adminv1.Service, error) {
	service, err := s.services.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return serviceResponse(service), nil
}

// CreateService 创建服务。
func (s *Service) CreateService(
	ctx context.Context,
	request *adminv1.CreateServiceRequest,
) (*adminv1.Service, error) {
	spec, err := parseServiceSpec(
		request.GetName(),
		request.GetEndpoints(),
		request.GetTls(),
		request.GetLoadBalancing(),
		request.GetHealthCheck(),
	)
	if err != nil {
		return nil, err
	}
	model, err := parseModelForCreate(request.GetModel())
	if err != nil {
		return nil, err
	}
	spec.Model = model

	service, err := s.services.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return serviceResponse(service), nil
}

// UpdateService 完整替换服务配置。
func (s *Service) UpdateService(
	ctx context.Context,
	request *adminv1.UpdateServiceRequest,
) (*adminv1.Service, error) {
	spec, err := parseServiceSpec(
		request.GetName(),
		request.GetEndpoints(),
		request.GetTls(),
		request.GetLoadBalancing(),
		request.GetHealthCheck(),
	)
	if err != nil {
		return nil, err
	}
	model, preserveAPIKey, err := parseModelForUpdate(request.GetModel())
	if err != nil {
		return nil, err
	}
	spec.Model = model

	input := servicebiz.ReplaceInput{
		ExpectedGeneration: request.GetVersion(),
		Spec:               spec,
		PreserveAPIKey:     preserveAPIKey,
	}
	service, err := s.services.Replace(ctx, request.GetId(), input)
	if err != nil {
		return nil, err
	}
	return serviceResponse(service), nil
}

// DeleteService 删除服务。
func (s *Service) DeleteService(
	ctx context.Context,
	request *adminv1.DeleteServiceRequest,
) (*emptypb.Empty, error) {
	if err := s.services.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
