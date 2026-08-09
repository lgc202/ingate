package registry

import (
	"context"
	"fmt"
	"path"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const displayNameLockTTL = 15

// DisplayNameGuard 在多个 API Server 实例之间串行化同类资源的展示名称写入
type DisplayNameGuard struct {
	client    *clientv3.Client
	keyPrefix string
}

// NewDisplayNameGuard 创建展示名称写入边界
func NewDisplayNameGuard(client *clientv3.Client, storagePrefix string) *DisplayNameGuard {
	return &DisplayNameGuard{
		client:    client,
		keyPrefix: path.Join(storagePrefix, "internal", "display-name-locks"),
	}
}

func (g *DisplayNameGuard) lock(
	ctx context.Context,
	resource schema.GroupResource,
	operation func() error,
) error {
	session, err := concurrency.NewSession(g.client, concurrency.WithContext(ctx), concurrency.WithTTL(displayNameLockTTL))
	if err != nil {
		return fmt.Errorf("create display name lock session: %w", err)
	}
	defer session.Close()

	mutex := concurrency.NewMutex(session, path.Join(g.keyPrefix, resource.Group, resource.Resource))
	if err := mutex.Lock(ctx); err != nil {
		return fmt.Errorf("acquire display name lock: %w", err)
	}
	// 关闭 session 会撤销租约并释放锁，不在成功写入后用 Unlock 失败覆盖业务结果
	return operation()
}
