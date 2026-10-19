package envoy

import (
	"context"
	"fmt"
	"slices"
	"sync"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

const sharedDataPlaneCluster = "ingate-shared-data-plane"

// Publisher 保存共享 Envoy 数据面的完整 xDS 配置。
type Publisher struct {
	cache           cachev3.SnapshotCache
	aiEgressSocket  string
	aiIngressSocket string
	// mu 串行化两类期望配置的合并与发布；保存最近构建有效的不可变期望资源。
	// SetSnapshot 的 Watch 通知错误不代表缓存未更新，不能据此回滚该状态。
	mu            sync.Mutex
	httpResources resourceSet
	aiResources   resourceSet
}

type clusterHash struct{}

// NewPublisher 创建按 Envoy node.cluster 分组的 ADS Snapshot Cache。
func NewPublisher(aiIngressSocket, aiEgressSocket string) *Publisher {
	return &Publisher{
		cache:           cachev3.NewSnapshotCache(true, clusterHash{}, nil),
		aiEgressSocket:  aiEgressSocket,
		aiIngressSocket: aiIngressSocket,
	}
}

// Publish 替换 HTTP 期望配置，并与最近构建有效的 AI 入口/出口合并为完整 xDS 视图。
// 返回成功表示 Cache 已更新，不表示 Envoy 已 ACK 或开始接流量。
func (p *Publisher) Publish(ctx context.Context, config dataplane.Config) error {
	resources, err := buildResources(config)
	if err != nil {
		return fmt.Errorf("build xDS resources: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.publish(ctx, resources, p.aiResources)
}

// PublishAI 替换模型入口和出口，保留最近成功的 HTTP 配置；合法空配置撤销 AI 资源。
// 构建失败保留已有结果；缓存安装后的通知错误保留新期望，交给协调器重试。
// 与 Publish 可并发调用，不保存调用方的可变集合，不回滚可能已经发出的 xDS。
func (p *Publisher) PublishAI(ctx context.Context, config aidataplane.Config) error {
	resources, err := buildAIResources(config, p.aiIngressSocket, p.aiEgressSocket)
	if err != nil {
		return fmt.Errorf("build AI xDS resources: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.publish(ctx, p.httpResources, resources)
}

func (p *Publisher) publish(ctx context.Context, httpResources, aiResources resourceSet) error {
	resources := resourceSet{
		listeners: append(slices.Clone(httpResources.listeners), aiResources.listeners...),
		routes:    append(mergeAIIngress(httpResources.routes, aiResources.ingressRoutes), aiResources.routes...),
		clusters:  append(slices.Clone(httpResources.clusters), aiResources.clusters...),
		endpoints: append(slices.Clone(httpResources.endpoints), aiResources.endpoints...),
	}
	snapshot, err := newSnapshot(resources)
	if err != nil {
		return fmt.Errorf("build xDS snapshot: %w", err)
	}

	var desired cachev3.ResourceSnapshot = snapshot
	current, err := p.cache.GetSnapshot(sharedDataPlaneCluster)
	if err == nil {
		if sameVersions(current, snapshot) {
			desired = current
		} else {
			reuseSnapshotResources(current, snapshot)
		}
	}

	// 原生 Cache 先安装再通知，通知可因取消失败且部分 Watch 已收到结果。
	// 记录同一份期望；即使版本未变也再次调用，以恢复上次失败后仍悬挂的 Watch。
	p.httpResources = httpResources
	p.aiResources = aiResources
	if err := p.cache.SetSnapshot(ctx, sharedDataPlaneCluster, desired); err != nil {
		return fmt.Errorf("publish xDS snapshot: %w", err)
	}

	return nil
}

// ID 使用 node.cluster，让同一数据面组的所有 Envoy 副本取得相同配置。
func (clusterHash) ID(node *corev3.Node) string {
	return node.GetCluster()
}
