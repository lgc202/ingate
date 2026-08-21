package registry

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// Strategy 提供所有集群级 Ingate 资源共用的 generic-apiserver 策略行为
//
// 资源自己的类型转换、默认值和校验仍由各 registry 子包实现
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy 创建不支持 update-create 和无条件更新的集群级资源策略
func NewStrategy(typer runtime.ObjectTyper) Strategy {
	return Strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

// NamespaceScoped 表示当前 Ingate 只维护一个配置域，资源不划分命名空间
func (Strategy) NamespaceScoped() bool {
	return false
}

// GetResetFields 阻止主资源接口直接写入 status
func (Strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return resetFields("status")
}

// WarningsOnCreate 当前没有需要通过 Kubernetes warning header 返回的提示
func (Strategy) WarningsOnCreate(context.Context, runtime.Object) []string {
	return nil
}

// AllowCreateOnUpdate 禁止通过更新不存在的 ID 隐式创建资源
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// WarningsOnUpdate 当前没有需要通过 Kubernetes warning header 返回的提示
func (Strategy) WarningsOnUpdate(context.Context, runtime.Object, runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate 要求更新请求携带当前 resourceVersion，避免覆盖并发修改
func (Strategy) AllowUnconditionalUpdate() bool {
	return false
}

// SpecResetFields 阻止 status 子资源接口修改 spec
func SpecResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return resetFields("spec")
}

func resetFields(field string) map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie(field),
		),
	}
}
