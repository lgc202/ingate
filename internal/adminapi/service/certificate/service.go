// Package certificate 提供 Certificate 管理 API。
package certificate

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现网关 TLS 证书管理 API。
type Service struct {
	certificates *certificatebiz.Usecase
}

// NewService 创建证书协议服务。
func NewService(certificates *certificatebiz.Usecase) *Service {
	return &Service{certificates: certificates}
}

// ListCertificates 返回满足筛选条件的证书列表。
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
	certificates := make([]*adminv1.Certificate, len(page.Items))
	for i := range page.Items {
		certificates[i] = certificateSummaryResponse(&page.Items[i])
	}
	return &adminv1.ListCertificatesResponse{
		Certificates: certificates,
		NextCursor:   page.NextCursor,
	}, nil
}

// GetCertificate 返回指定证书及其证书链。
func (s *Service) GetCertificate(
	ctx context.Context,
	request *adminv1.GetCertificateRequest,
) (*adminv1.Certificate, error) {
	certificate, err := s.certificates.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return certificateDetailResponse(certificate), nil
}

// CreateCertificate 创建证书。
func (s *Service) CreateCertificate(
	ctx context.Context,
	request *adminv1.CreateCertificateRequest,
) (*adminv1.Certificate, error) {
	spec, err := parseCertificateSpec(
		request.GetName(),
		request.GetCertificatePem(),
		request.GetPrivateKeyPem(),
	)
	if err != nil {
		return nil, err
	}
	certificate, err := s.certificates.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return certificateDetailResponse(certificate), nil
}

// UpdateCertificate 完整替换证书配置；省略证书链和私钥时保留已有密钥对。
func (s *Service) UpdateCertificate(
	ctx context.Context,
	request *adminv1.UpdateCertificateRequest,
) (*adminv1.Certificate, error) {
	spec, preserveKeyPair, err := parseCertificateReplacement(request)
	if err != nil {
		return nil, err
	}
	input := certificatebiz.ReplaceInput{
		ExpectedGeneration: request.GetVersion(),
		Spec:               spec,
		PreserveKeyPair:    preserveKeyPair,
	}
	certificate, err := s.certificates.Replace(ctx, request.GetId(), input)
	if err != nil {
		return nil, err
	}
	return certificateDetailResponse(certificate), nil
}

// DeleteCertificate 删除证书。
func (s *Service) DeleteCertificate(
	ctx context.Context,
	request *adminv1.DeleteCertificateRequest,
) (*emptypb.Empty, error) {
	if err := s.certificates.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
