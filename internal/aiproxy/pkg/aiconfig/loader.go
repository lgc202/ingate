// Package aiconfig 加载、发布并提供 AI Proxy 请求使用的只读运行配置。
// 通过原生 ConfigMap List/Watch 接收共享编译配置，校验后建立查询索引。
// 通信失败和同实例的坏更新保留上次有效值；确认删除和实例替换撤回旧值。
// 本包不连接模型后端，不表示流量就绪，也不拥有 API 客户端的生命周期。
package aiconfig

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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// Loader 持有单一 ConfigMap 实例的完整有效快照，不能复制或重复运行。
// Capture 只交付不可变运行配置，不暴露编译配置中的可变数据。
type Loader struct {
	client    kubernetes.Interface
	namespace string
	logger    *slog.Logger
	started   atomic.Bool

	// mu 保护 uid 与 config 指针的读取、替换和撤回，不保护 Config 内部字段。
	// Config 发布后保持只读，因此 Capture 解锁后仍可使用已捕获的实例。
	mu     sync.RWMutex
	uid    types.UID
	config *Config
}

// NewLoader 保存已经校验的加载范围和已构造的依赖，不发起请求、不启动后台任务。
// namespace 由组件配置入口校验，API Client 与 Logger 由组合根成功构造后注入。
func NewLoader(
	apiClient kubernetes.Interface,
	namespace string,
	logger *slog.Logger,
) *Loader {
	return &Loader{
		client:    apiClient,
		namespace: namespace,
		logger:    logger.With("namespace", namespace, "name", aidataplane.ConfigMapName),
	}
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

// Capture 为一次请求捕获当前完整运行配置；尚未加载或已撤回时返回 false。
// 调用方应在请求开始时捕获一次，不能在处理途中重新捕获并混用版本。
//
// 本方法只复制配置指针，不深拷贝配置内容。读锁保证取指针时不会与发布、撤回并发；
// 解锁后的安全性依赖 Config 发布后不再修改，而不是继续持有此锁。
// 后续更新只替换 Loader 的指针，已捕获的指针仍指向原来的完整实例。
//
// 有效更新后捕获新实例；确认删除或停止后没有可捕获实例。
// 这些变化不修改已经交付给在途请求的实例。
func (l *Loader) Capture() (*Config, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.config, l.config != nil
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

	// 同名新 UID 是另一个对象实例，先撤回旧值；即使新内容校验失败也不能继承旧配置。
	l.mu.Lock()
	if l.uid != configMap.UID {
		l.uid = configMap.UID
		l.config = nil
	}

	l.mu.Unlock()

	if _, occupied := configMap.BinaryData[aidataplane.ConfigMapDataKey]; occupied {
		return fmt.Errorf("AI configuration key %q is occupied by binaryData", aidataplane.ConfigMapDataKey)
	}

	data, exists := configMap.Data[aidataplane.ConfigMapDataKey]
	if !exists {
		return fmt.Errorf("AI configuration key %q is missing", aidataplane.ConfigMapDataKey)
	}

	// 单个 Informer 处理器串行调用 replace/remove；clear 在 Shutdown 等待处理器退出后执行。
	// 因此构建期间不会有其他写入者抢先发布或撤回，无须持锁解码、建立查找表。
	// 同 UID 更新期间，请求仍可捕获上一次有效实例；新 UID 的旧值已在上面撤回。
	config, err := newConfig([]byte(data))
	if err != nil {
		return fmt.Errorf("decode AI configuration ConfigMap: %w", err)
	}

	// 与 Capture 的读锁配合，只发布已经构建完成的实例；不原地修改旧实例的内容。
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.config == nil || l.config.Revision() != config.Revision() {
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
