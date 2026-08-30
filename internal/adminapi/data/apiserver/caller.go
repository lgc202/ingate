package apiserver

import (
	"context"

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

// ListByIDs 返回当前存在的指定 Caller。
func (s *CallerStore) ListByIDs(
	ctx context.Context,
	callerIDs []string,
) (map[string]*resource.Caller, error) {
	return listByIDs(ctx, callerIDs, s.Get)
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
func (s *CallerStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Caller,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	return replaceResourceSpec(
		ctx,
		s.client.GatewayV1().Callers(),
		"caller",
		observed,
		func(caller *resource.Caller) { caller.Spec = spec },
	)
}

// Delete 删除 Caller。
func (s *CallerStore) Delete(
	ctx context.Context,
	observed *resource.Caller,
) error {
	return deleteResource(
		ctx,
		s.client.GatewayV1().Callers(),
		"caller",
		observed,
	)
}
