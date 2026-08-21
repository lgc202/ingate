package registry

import (
	"maps"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// PrepareObjectMetaForCreate 初始化由 API Server 维护的资源版本元数据
func PrepareObjectMetaForCreate(metadata *metav1.ObjectMeta) {
	metadata.Generation = 1
	setUpdatedAt(metadata, metadata.CreationTimestamp.Time)
}

// PrepareObjectMetaForUpdate 只在期望配置变化时推进 Generation 和更新时间
func PrepareObjectMetaForUpdate(metadata, oldMetadata *metav1.ObjectMeta, specChanged bool) {
	metadata.Generation = oldMetadata.Generation
	if specChanged {
		metadata.Generation++
		setUpdatedAt(metadata, time.Now().UTC())
		return
	}
	preserveUpdatedAt(metadata, oldMetadata)
}

func setUpdatedAt(metadata *metav1.ObjectMeta, updatedAt time.Time) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt.Format(time.RFC3339Nano)
}

func preserveUpdatedAt(metadata, oldMetadata *metav1.ObjectMeta) {
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
