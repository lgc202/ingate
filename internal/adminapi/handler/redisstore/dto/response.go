package dto

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	redisstoreservice "github.com/lgc202/ingate/internal/adminapi/service/redisstore"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListRedisStoresResp 转换 RedisStore 列表用例结果为控制台响应
func NewListRedisStoresResp(result *redisstoreservice.ListResult) ListRedisStoresResp {
	return ListRedisStoresResp{
		RedisStores: lo.Map(result.RedisStores, func(store resource.RedisStore, _ int) RedisStore {
			return redisStoreFromResource(&store)
		}),
	}
}

// NewGetRedisStoreResp 转换单个 RedisStore 用例结果为控制台响应
func NewGetRedisStoreResp(result *redisstoreservice.RedisStoreResult) RedisStore {
	return redisStoreFromResource(result.RedisStore)
}

func redisStoreFromResource(store *resource.RedisStore) RedisStore {
	return RedisStore{
		ID:      store.Name,
		Version: store.ResourceVersion,
		RedisStoreConfig: RedisStoreConfig{
			Name:                 store.Spec.DisplayName,
			Description:          store.Spec.Description,
			Mode:                 store.Spec.Mode,
			Address:              store.Spec.Address,
			DB:                   store.Spec.DB,
			TLS:                  store.Spec.TLS,
			Username:             store.Spec.Username,
			PasswordRef:          store.Spec.PasswordRef,
			ConnectTimeoutMillis: store.Spec.ConnectTimeoutMillis,
			CommandTimeoutMillis: store.Spec.CommandTimeoutMillis,
		},
		CreatedAt: createdAt(store.ObjectMeta),
	}
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
