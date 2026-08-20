package registry

import (
	"maps"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// SetUpdatedAt 记录最近一次期望配置发生变化的时间
func SetUpdatedAt(metadata *metav1.ObjectMeta, updatedAt time.Time) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt.Format(time.RFC3339Nano)
}

// PreserveUpdatedAt 阻止客户端通过无配置变化的更新覆盖系统维护的更新时间
func PreserveUpdatedAt(metadata, oldMetadata *metav1.ObjectMeta) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	delete(metadata.Annotations, resource.AnnotationUpdatedAt)

	updatedAt := oldMetadata.Annotations[resource.AnnotationUpdatedAt]
	if updatedAt == "" {
		return
	}
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt
}
