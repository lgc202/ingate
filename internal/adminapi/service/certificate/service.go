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
	business *certificatebiz.Service
}

// NewService 创建证书协议服务
func NewService(business *certificatebiz.Service) *Service {
	return &Service{business: business}
}

func (s *Service) ListCertificates(ctx context.Context, request *adminv1.ListCertificatesRequest) (*adminv1.ListCertificatesResponse, error) {
	result, err := s.business.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListCertificatesResponse{
		Certificates: make([]*adminv1.Certificate, 0, len(result.Items)),
		NextCursor:   result.NextCursor,
	}
	for i := range result.Items {
		response.Certificates = append(response.Certificates, certificateFromResource(&result.Items[i]))
	}
	return response, nil
}

func (s *Service) GetCertificate(ctx context.Context, request *adminv1.GetCertificateRequest) (*adminv1.Certificate, error) {
	item, err := s.business.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return certificateWithPEMFromResource(item), nil
}

func (s *Service) CreateCertificate(ctx context.Context, request *adminv1.CreateCertificateRequest) (*adminv1.Certificate, error) {
	certificatePEM := request.GetCertificatePem()
	privateKeyPEM := request.GetPrivateKeyPem()
	spec, err := buildCertificateSpec(request.GetName(), &certificatePEM, &privateKeyPEM)
	if err != nil {
		return nil, err
	}
	item, err := s.business.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return certificateWithPEMFromResource(item), nil
}

func (s *Service) UpdateCertificate(ctx context.Context, request *adminv1.UpdateCertificateRequest) (*adminv1.Certificate, error) {
	spec, err := buildCertificateSpec(request.GetName(), request.CertificatePem, request.PrivateKeyPem)
	if err != nil {
		return nil, err
	}
	item, err := s.business.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return certificateWithPEMFromResource(item), nil
}

func (s *Service) DeleteCertificate(ctx context.Context, request *adminv1.DeleteCertificateRequest) (*emptypb.Empty, error) {
	if err := s.business.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
