package apiserver

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// CallerStore 读写 Caller 声明式资源。
type CallerStore struct {
	client clientset.Interface
}

// NewCallerStore 创建 Caller Store。
func NewCallerStore(client clientset.Interface) *CallerStore {
	return &CallerStore{client: client}
}

// ListPage 分页返回 Caller。
func (s *CallerStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.Caller], error) {
	callers, err := s.client.GatewayV1().Callers().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Caller]{}, listError("callers", err)
	}
	return biz.PageResult[resource.Caller]{
		Items:      callers.Items,
		NextCursor: callers.Continue,
	}, nil
}

// Get 返回指定 Caller。
func (s *CallerStore) Get(ctx context.Context, callerID string) (*resource.Caller, error) {
	caller, err := s.client.GatewayV1().Callers().Get(ctx, callerID, metav1.GetOptions{})
	return caller, resourceError("get", "caller", callerID, err)
}

// Create 创建 Caller。
func (s *CallerStore) Create(
	ctx context.Context,
	callerID string,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	caller := &resource.Caller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindCaller),
		},
		ObjectMeta: metav1.ObjectMeta{Name: callerID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().Callers().Create(ctx, caller, metav1.CreateOptions{})
	return created, resourceError("create", "caller", callerID, err)
}

// ReplaceSpec 完整替换 Caller 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *CallerStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Caller,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	callerID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().Callers(),
		observed,
		func(caller *resource.Caller) { caller.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf("replace caller %q after conflict retries: %w", callerID, err)
	}
	return updated, resourceError("replace", "caller", callerID, err)
}

// Delete 删除 Caller。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *CallerStore) Delete(
	ctx context.Context,
	observed *resource.Caller,
) error {
	callerID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().Callers(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf("delete caller %q after conflict retries: %w", callerID, err)
	}
	return resourceError("delete", "caller", callerID, err)
}
