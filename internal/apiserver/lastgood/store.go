// Package lastgood 在 apiserver 内部持久化 Envoy Last Good 配置
package lastgood

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"slices"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apiserver/pkg/storage/storagebackend"

	envoylastgood "github.com/lgc202/ingate/internal/envoy/lastgood"
)

// Store 使用 apiserver 的 etcd 配置保存唯一一份 Envoy Last Good
type Store struct {
	client *clientv3.Client
}

// NewStore 使用 apiserver 的 etcd 连接配置创建 Last Good store
func NewStore(config storagebackend.TransportConfig) (*Store, error) {
	tlsConfig, err := newTLSConfig(config)
	if err != nil {
		return nil, err
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints: slices.Clone(config.ServerList),
		TLS:       tlsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create last good etcd client: %w", err)
	}
	return &Store{client: client}, nil
}

func (s *Store) load(ctx context.Context) (envoylastgood.Record, error) {
	response, err := s.client.Get(ctx, envoylastgood.Key)
	if err != nil {
		return envoylastgood.Record{}, fmt.Errorf("load last good from etcd: %w", err)
	}
	if len(response.Kvs) == 0 {
		return envoylastgood.Record{}, envoylastgood.ErrNotFound
	}

	record, err := envoylastgood.Decode(bytes.NewReader(response.Kvs[0].Value))
	if err != nil {
		return envoylastgood.Record{}, err
	}
	return record, nil
}

func (s *Store) save(ctx context.Context, record envoylastgood.Record) error {
	data, err := envoylastgood.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := s.client.Put(ctx, envoylastgood.Key, string(data)); err != nil {
		return fmt.Errorf("save last good to etcd: %w", err)
	}
	return nil
}

// Close 关闭 Store 持有的 etcd client
func (s *Store) Close() error {
	return s.client.Close()
}

func newTLSConfig(config storagebackend.TransportConfig) (*tls.Config, error) {
	if config.CertFile == "" && config.KeyFile == "" && config.TrustedCAFile == "" {
		return nil, nil
	}

	tlsConfig, err := (transport.TLSInfo{
		CertFile:      config.CertFile,
		KeyFile:       config.KeyFile,
		TrustedCAFile: config.TrustedCAFile,
	}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("create last good etcd TLS config: %w", err)
	}
	return tlsConfig, nil
}
