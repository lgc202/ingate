package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// CertificateStore 读写 Certificate 声明式资源。
type CertificateStore struct {
	*resourceStore[resource.Certificate, *resource.Certificate, resource.CertificateSpec]
}

// NewCertificateStore 创建 Certificate Store。
func NewCertificateStore(client clientset.Interface) *CertificateStore {
	return &CertificateStore{resourceStore: newResourceStore(
		"certificate",
		"certificates",
		func() createResourceClient[*resource.Certificate] {
			return client.GatewayV1().Certificates()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Certificate, string, error) {
			resources := client.GatewayV1().Certificates()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.CertificateSpec) *resource.Certificate {
			return &resource.Certificate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindCertificate),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Certificate, spec resource.CertificateSpec) { object.Spec = spec },
	)}
}
