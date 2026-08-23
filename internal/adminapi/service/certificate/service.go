// Package certificate 提供 Certificate 管理 API
package certificate

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现网关 TLS 证书管理 API
type Service struct {
	certificates *certificatebiz.Service
}

// NewService 创建证书协议服务
func NewService(certificates *certificatebiz.Service) *Service {
	return &Service{certificates: certificates}
}

func (s *Service) ListCertificates(
	ctx context.Context,
	request *adminv1.ListCertificatesRequest,
) (*adminv1.ListCertificatesResponse, error) {
	page, err := s.certificates.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), nil, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListCertificatesResponse{
		Certificates: make([]*adminv1.Certificate, 0, len(page.Items)),
		NextCursor:   page.NextCursor,
	}
	for i := range page.Items {
		response.Certificates = append(response.Certificates, certificateResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetCertificate(ctx context.Context, request *adminv1.GetCertificateRequest) (*adminv1.Certificate, error) {
	certificate, err := s.certificates.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return certificateWithPEMResponse(certificate), nil
}

func (s *Service) CreateCertificate(ctx context.Context, request *adminv1.CreateCertificateRequest) (*adminv1.Certificate, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	certificate, err := s.certificates.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return certificateWithPEMResponse(certificate), nil
}

func (s *Service) UpdateCertificate(ctx context.Context, request *adminv1.UpdateCertificateRequest) (*adminv1.Certificate, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	certificate, err := s.certificates.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return certificateWithPEMResponse(certificate), nil
}

func (s *Service) DeleteCertificate(ctx context.Context, request *adminv1.DeleteCertificateRequest) (*emptypb.Empty, error) {
	if err := s.certificates.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
