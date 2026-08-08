package service

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// CertificateService 实现网关 TLS 证书管理 API
type CertificateService struct {
	usecase *biz.CertificateUsecase
}

// NewCertificateService 创建证书协议服务
func NewCertificateService(usecase *biz.CertificateUsecase) *CertificateService {
	return &CertificateService{usecase: usecase}
}

func (s *CertificateService) ListCertificates(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListCertificatesReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询证书失败")
	}
	reply := &adminv1.ListCertificatesReply{Certificates: make([]*adminv1.Certificate, 0, len(items))}
	for i := range items {
		reply.Certificates = append(reply.Certificates, certificateReply(&items[i], false))
	}
	return reply, nil
}

func (s *CertificateService) GetCertificate(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.GetCertificateReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询证书失败")
	}
	return &adminv1.GetCertificateReply{Certificate: certificateReply(item, true)}, nil
}

func (s *CertificateService) CreateCertificate(ctx context.Context, request *adminv1.CreateCertificateRequest) (*adminv1.MutationReply, error) {
	spec, err := certificateSpec(request.GetName(), request.GetDescription(), request.GetCertificatePem(), request.GetPrivateKeyPem(), true)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建证书失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *CertificateService) UpdateCertificate(ctx context.Context, request *adminv1.UpdateCertificateRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if request.GetVersion() == "" {
		return nil, badRequest("证书版本不能为空")
	}
	spec, err := certificateSpec(request.GetName(), request.GetDescription(), request.GetCertificatePem(), request.GetPrivateKeyPem(), false)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, operationError(err, "更新证书失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *CertificateService) DeleteCertificate(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除证书失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func certificateSpec(name, description, certificatePEM, privateKeyPEM string, creating bool) (resource.CertificateSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.CertificateSpec{}, badRequest("证书名称不能为空")
	}
	certificatePEM = normalizePEM(certificatePEM)
	privateKeyPEM = normalizePEM(privateKeyPEM)
	if creating && (certificatePEM == "" || privateKeyPEM == "") {
		return resource.CertificateSpec{}, badRequest("证书内容和私钥不能为空")
	}
	if !creating && (certificatePEM == "") != (privateKeyPEM == "") {
		return resource.CertificateSpec{}, badRequest("替换证书时必须同时提供证书内容和私钥")
	}
	if certificatePEM != "" {
		if _, err := certificateutil.ParseKeyPair(certificatePEM, privateKeyPEM); err != nil {
			return resource.CertificateSpec{}, badRequest("证书内容与私钥格式不正确或不匹配")
		}
	}
	return resource.CertificateSpec{
		DisplayName: name, Description: strings.TrimSpace(description),
		CertificatePEM: certificatePEM, PrivateKeyPEM: privateKeyPEM,
	}, nil
}

func certificateReply(certificate *resource.Certificate, includePEM bool) *adminv1.Certificate {
	reply := &adminv1.Certificate{
		Id:          certificate.Name,
		Version:     strconv.FormatInt(certificate.Generation, 10),
		Status:      resourceStatus(biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions)),
		Name:        certificate.Spec.DisplayName,
		Description: certificate.Spec.Description,
		DnsNames:    []string{},
		CreatedAt:   timestamp(certificate.CreationTimestamp.Time),
	}
	if includePEM {
		reply.CertificatePem = certificate.Spec.CertificatePEM
	}
	leaf, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM)
	if err == nil {
		reply.DnsNames = append([]string(nil), leaf.DNSNames...)
		reply.NotBefore = timestamp(leaf.NotBefore)
		reply.NotAfter = timestamp(leaf.NotAfter)
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
