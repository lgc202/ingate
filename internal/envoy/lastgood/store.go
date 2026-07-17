package lastgood

import (
	"context"
	"encoding/json"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Store 使用单个 etcd key 保存 Last Good
type Store struct {
	kv clientv3.KV
}

// NewStore 创建 Last Good store
func NewStore(kv clientv3.KV) *Store {
	return &Store{kv: kv}
}

// Load 读取并校验 Last Good 记录
func (s *Store) Load(ctx context.Context) (Record, error) {
	response, err := s.kv.Get(ctx, Key)
	if err != nil {
		return Record{}, fmt.Errorf("load last good from etcd: %w", err)
	}
	if len(response.Kvs) == 0 {
		return Record{}, ErrNotFound
	}

	var record Record
	if err := json.Unmarshal(response.Kvs[0].Value, &record); err != nil {
		return Record{}, fmt.Errorf("%w: decode record: %v", ErrCorrupt, err)
	}
	if _, err := record.Config(); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Save 完整校验记录后通过单次 Put 覆盖 Last Good
func (s *Store) Save(ctx context.Context, record Record) error {
	if _, err := record.Config(); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode last good record: %w", err)
	}
	if _, err := s.kv.Put(ctx, Key, string(data)); err != nil {
		return fmt.Errorf("save last good to etcd: %w", err)
	}
	return nil
}
