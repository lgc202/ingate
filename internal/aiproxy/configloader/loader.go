// Package configloader 通过原生 ConfigMap List/Watch 加载共享 AI 配置。
// 通信失败和同实例的坏更新保留上次有效值；确认删除和实例替换撤回旧值。
// 本包不连接模型后端，不表示流量就绪，也不拥有 API 客户端的生命周期。
package configloader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// Loader 持有单一 ConfigMap 实例的完整有效快照，不能复制或重复运行。
// 快照保持私有；实际消费接口在 Proxy 的调用路径确定后再添加。
type Loader struct {
	client    kubernetes.Interface
	namespace string
	logger    *slog.Logger
	started   atomic.Bool

	mu     sync.RWMutex
	uid    types.UID
	config *aidataplane.Config
}

// New 校验加载范围并保存依赖，不发起请求、不启动后台任务。
func New(
	apiClient kubernetes.Interface,
	namespace string,
	logger *slog.Logger,
) (*Loader, error) {
	if apiClient == nil {
		return nil, errors.New("AI configuration client is required")
	}
	if messages := validation.IsDNS1123Label(namespace); len(messages) != 0 {
		return nil, fmt.Errorf("AI configuration namespace %q must be a valid DNS label", namespace)
	}
	if logger == nil {
		return nil, errors.New("AI configuration logger is required")
	}

	return &Loader{
		client:    apiClient,
		namespace: namespace,
		logger:    logger.With("namespace", namespace, "name", aidataplane.ConfigMapName),
	}, nil
}

// Run 同步运行一次加载器，取消后等待 Informer 及其处理器停止并清空快照。
// 请求失败由 client-go 重试；坏配置只记录失败，不结束等待有效更新的过程。
// 本方法不以缓存同步成功或配置加载成功宣告数据面就绪。
func (l *Loader) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run AI configuration loader: %w", err)
	}
	if !l.started.CompareAndSwap(false, true) {
		return errors.New("AI configuration loader has already run")
	}
	defer l.clear()

	// 本 Factory 只管理目标 ConfigMap；这里的名称筛选会应用于其中所有 Informer。
	factory := informers.NewSharedInformerFactoryWithOptions(
		l.client,
		0,
		informers.WithNamespace(l.namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", aidataplane.ConfigMapName).String()
		}),
	)
	defer factory.Shutdown()

	// 在 Start 前实例化并注册；Factory 只启动已经请求的 Informer。
	informer := factory.Core().V1().ConfigMaps().TypedInformer()
	_, err := informer.AddTypedEventHandler(cache.TypedResourceEventHandlerFuncs[*corev1.ConfigMap]{
		AddFunc:    func(configMap *corev1.ConfigMap) { l.load(ctx, configMap) },
		UpdateFunc: func(_, current *corev1.ConfigMap) { l.load(ctx, current) },
		DeleteFunc: func(deleted cache.DeletedObject[*corev1.ConfigMap]) {
			if err := l.remove(deleted.OptionalObj); err != nil {
				l.logger.ErrorContext(ctx, "Failed to remove AI configuration", "err", err)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("register AI configuration handler: %w", err)
	}

	if err := informer.SetWatchErrorHandlerWithContext(func(ctx context.Context, _ *cache.Reflector, err error) {
		if ctx.Err() == nil {
			l.logger.WarnContext(ctx, "AI configuration watch failed; retaining last valid configuration", "err", err)
		}
	}); err != nil {
		return fmt.Errorf("register AI configuration watch error handler: %w", err)
	}

	factory.StartWithContext(ctx)
	<-ctx.Done()

	// Shutdown 只等待退出，不负责取消；它先于 clear 执行，避免迟到处理覆盖清空。
	return fmt.Errorf("run AI configuration loader: %w", ctx.Err())
}

func (l *Loader) load(ctx context.Context, configMap *corev1.ConfigMap) {
	if err := l.replace(configMap); err != nil {
		l.logger.ErrorContext(ctx, "Failed to load AI configuration", "err", err)
	}
}

// replace 在校验成功后完整交换快照；同名新 UID 不继承旧实例的有效值。
// 单个 Informer 处理器按顺序调用本方法，锁保护完整状态登记。
func (l *Loader) replace(configMap *corev1.ConfigMap) error {
	if configMap == nil {
		return errors.New("AI configuration event is missing its ConfigMap")
	}
	if !l.matches(configMap) {
		return nil
	}
	if configMap.UID == "" {
		return errors.New("AI configuration ConfigMap UID is missing")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.uid != configMap.UID {
		l.uid = configMap.UID
		l.config = nil
	}

	if _, occupied := configMap.BinaryData[aidataplane.ConfigMapDataKey]; occupied {
		return fmt.Errorf("AI configuration key %q is occupied by binaryData", aidataplane.ConfigMapDataKey)
	}
	data, exists := configMap.Data[aidataplane.ConfigMapDataKey]
	if !exists {
		return fmt.Errorf("AI configuration key %q is missing", aidataplane.ConfigMapDataKey)
	}

	config, err := aidataplane.Decode([]byte(data))
	if err != nil {
		return fmt.Errorf("decode AI configuration ConfigMap: %w", err)
	}

	if l.config == nil || l.config.Revision != config.Revision {
		l.config = config
	}

	return nil
}

// remove 也处理成功重新 List 后的缺失通知；旧实例的迟到删除不能撤销新实例。
func (l *Loader) remove(configMap *corev1.ConfigMap) error {
	if configMap == nil || configMap.UID == "" {
		return errors.New("AI configuration deletion is missing its object identity")
	}
	if !l.matches(configMap) {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.uid == configMap.UID {
		l.uid = ""
		l.config = nil
	}

	return nil
}

func (l *Loader) matches(configMap *corev1.ConfigMap) bool {
	return configMap.Namespace == l.namespace && configMap.Name == aidataplane.ConfigMapName
}

func (l *Loader) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.uid = ""
	l.config = nil
}
