// Package xds 组合 Envoy 与 AI Proxy 配置缓存，提供共享数据面组的 Delta ADS 分发。
package xds

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane/envoy"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

const sharedDataPlaneCluster = dataplane.SharedDataPlaneCluster

// Publisher 将同一次 AI 编译的两类输出分别安装到原生 SnapshotCache 和 AI Proxy LinearCache。
// 缓存安装不是跨代理事务，也不表示客户端已经 ACK 或后端已可用。
type Publisher struct {
	// mu 保证并发 AI 发布在两个缓存中采用同一安装顺序；不构成跨代理原子提交。
	mu          sync.Mutex
	envoy       *envoy.Publisher
	aiResources *aiResourceCache
	cache       *cachev3.MuxCache
}

// NewPublisher 创建两类内存缓存，不监听网络或启动后台任务。
func NewPublisher(aiIngressSocket, aiEgressSocket string) *Publisher {
	p := &Publisher{
		envoy:       envoy.NewPublisher(aiIngressSocket, aiEgressSocket),
		aiResources: newAIResourceCache(),
	}
	p.cache = &cachev3.MuxCache{
		Classify: func(request *cachev3.Request) string {
			return p.classify(request.GetTypeUrl(), request.GetNode().GetCluster())
		},
		ClassifyDelta: func(request *cachev3.DeltaRequest) string {
			return p.classify(request.GetTypeUrl(), request.GetNode().GetCluster())
		},
		Caches: map[string]cachev3.Cache{
			"envoy":   p.envoy.DiscoveryCache(),
			"aiproxy": p.aiResources.cache,
		},
	}

	return p
}

// Publish 更新普通 HTTP 输出，不改动 AI Proxy 资源。
func (p *Publisher) Publish(ctx context.Context, config dataplane.Config) error {
	return p.envoy.Publish(ctx, config)
}

// PublishAI 独立尝试两个缓存的安装；失败交给协调器对最新期望重试，不回滚已发出的结果。
// resources 必须来自同一次编译的 config；调用期间不能修改，返回后可以复用调用方存储。
func (p *Publisher) PublishAI(
	ctx context.Context,
	config aidataplane.Config,
	resources []*aidataplanev1.Resource,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	envoyErr := p.envoy.PublishAI(ctx, config)
	if envoyErr != nil {
		envoyErr = fmt.Errorf("publish Envoy AI resources: %w", envoyErr)
	}

	aiErr := p.aiResources.publish(resources)
	if aiErr != nil {
		aiErr = fmt.Errorf("publish AI Proxy resources: %w", aiErr)
	}

	return errors.Join(envoyErr, aiErr)
}

func (p *Publisher) checkReady() error {
	if err := p.envoy.CheckReady(); err != nil {
		return err
	}

	if !p.aiResources.initialized.Load() {
		return errors.New("initial AI Proxy configuration is unavailable")
	}

	return nil
}

// classify 按类型分流而非读取客户端自报的代理角色；这不是服务间鉴权。
// AI Proxy 首次编译前拒绝订阅，避免将未初始化的空缓存解释为有效空配置。
func (p *Publisher) classify(typeURL, cluster string) string {
	if cluster != sharedDataPlaneCluster || !supportedResourceType(typeURL) {
		return ""
	}

	if typeURL == aidataplanev1.ResourceType {
		if !p.aiResources.initialized.Load() {
			return ""
		}

		return "aiproxy"
	}

	return "envoy"
}
