// Package certificate 实现 Certificate 管理 API
package certificate

import (
	"context"
	"strconv"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 实现网关 TLS 证书管理 API
type Service struct {
	usecase *certificatebiz.Usecase
}

// NewService 创建证书协议服务
func NewService(usecase *certificatebiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListCertificates(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListCertificatesReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetPageSize(), request.GetPageToken()))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListCertificatesReply{Certificates: make([]*adminv1.Certificate, 0, len(result.Items)), Page: adminservice.PageInfo(result.NextCursor)}
	for i := range result.Items {
		reply.Certificates = append(reply.Certificates, newCertificateReply(&result.Items[i], false))
	}
	return reply, nil
}

func (s *Service) GetCertificate(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.GetCertificateReply, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return &adminv1.GetCertificateReply{Certificate: newCertificateReply(item, true)}, nil
}

func (s *Service) CreateCertificate(ctx context.Context, request *adminv1.CreateCertificateRequest) (*adminv1.MutationReply, error) {
	spec, err := buildCertificateSpec(request.GetName(), request.GetDescription(), request.GetCertificatePem(), request.GetPrivateKeyPem(), true)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateCertificate(ctx context.Context, request *adminv1.UpdateCertificateRequest) (*adminv1.MutationReply, error) {
	spec, err := buildCertificateSpec(request.GetName(), request.GetDescription(), request.GetCertificatePem(), request.GetPrivateKeyPem(), false)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteCertificate(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func buildCertificateSpec(name, description, certificatePEM, privateKeyPEM string, creating bool) (resource.CertificateSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书名称不能为空")
	}
	certificatePEM = normalizePEM(certificatePEM)
	privateKeyPEM = normalizePEM(privateKeyPEM)
	if creating && (certificatePEM == "" || privateKeyPEM == "") {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书内容和私钥不能为空")
	}
	if !creating && (certificatePEM == "") != (privateKeyPEM == "") {
		return resource.CertificateSpec{}, adminservice.BadRequest("替换证书时必须同时提供证书内容和私钥")
	}
	if certificatePEM != "" {
		if _, err := certificateutil.ParseKeyPair(certificatePEM, privateKeyPEM); err != nil {
			return resource.CertificateSpec{}, adminservice.BadRequest("证书内容与私钥格式不正确或不匹配")
		}
	}
	return resource.CertificateSpec{
		DisplayName: name, Description: strings.TrimSpace(description),
		CertificatePEM: certificatePEM, PrivateKeyPEM: privateKeyPEM,
	}, nil
}

func newCertificateReply(certificate *resource.Certificate, includePEM bool) *adminv1.Certificate {
	reply := &adminv1.Certificate{
		Id:          certificate.Name,
		Version:     strconv.FormatInt(certificate.Generation, 10),
		Status:      adminservice.NewResourceStatus(biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions)),
		Name:        certificate.Spec.DisplayName,
		Description: certificate.Spec.Description,
		DnsNames:    []string{},
		CreatedAt:   adminservice.NewTimestamp(certificate.CreationTimestamp.Time),
	}
	if includePEM {
		reply.CertificatePem = certificate.Spec.CertificatePEM
	}
	leaf, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM)
	if err == nil {
		reply.DnsNames = append([]string(nil), leaf.DNSNames...)
		reply.NotBefore = adminservice.NewTimestamp(leaf.NotBefore)
		reply.NotAfter = adminservice.NewTimestamp(leaf.NotAfter)
	}
	return reply
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}
