package gateway

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	aiConfigMapName        = "ingate-shared-data-plane-ai"
	aiConfigDataKey        = "config.json"
	aiConfigManagedByKey   = "app.kubernetes.io/managed-by"
	aiConfigManagedByValue = "ingate-controller-manager"
)

// aiConfigWriter 只保存一份共享配置，不拥有客户端或 Controller 生命周期。
// reader 接收 Manager 的 APIReader，避免用陈旧 Cache 结果覆盖其他写入。
type aiConfigWriter struct {
	client    client.Client
	reader    client.Reader
	namespace string
}

// write 保存 compileAI 成功返回的完整 JSON，不再次解析领域资源或 payload。
// Namespace 必须由部署显式提供并预先创建；空字节不是撤销配置。
// Conflict 和 AlreadyExists 保留给协调器重读并重试，不强删重建对象。
func (w *aiConfigWriter) write(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return errors.New("compiled AI configuration must not be empty")
	}
	if len(data) > corev1.MaxSecretSize {
		return fmt.Errorf("compiled AI configuration data size %d exceeds %d bytes", len(data), corev1.MaxSecretSize)
	}
	if messages := validation.IsDNS1123Label(w.namespace); len(messages) != 0 {
		return fmt.Errorf("AI configuration namespace %q must be a valid DNS label", w.namespace)
	}

	var namespace corev1.Namespace
	if err := w.reader.Get(ctx, client.ObjectKey{Name: w.namespace}, &namespace); err != nil {
		return fmt.Errorf("get AI configuration Namespace %q: %w", w.namespace, err)
	}
	if !namespace.DeletionTimestamp.IsZero() || namespace.Status.Phase == corev1.NamespaceTerminating {
		return fmt.Errorf("AI configuration Namespace %q is terminating", w.namespace)
	}

	key := client.ObjectKey{Namespace: w.namespace, Name: aiConfigMapName}
	var (
		configMap corev1.ConfigMap
		create    bool
	)
	if err := w.reader.Get(ctx, key, &configMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get AI configuration ConfigMap %q: %w", key.String(), err)
		}
		create = true
	}

	if create {
		configMap = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: w.namespace,
				Name:      aiConfigMapName,
				Labels:    map[string]string{aiConfigManagedByKey: aiConfigManagedByValue},
			},
		}
	} else {
		// 标签是受管对象约定，不是服务身份或授权校验。
		if configMap.Labels[aiConfigManagedByKey] != aiConfigManagedByValue {
			return fmt.Errorf(
				"AI configuration ConfigMap %q ownership conflict: %w",
				key.String(),
				apierrors.NewConflict(
					schema.GroupResource{Resource: "configmaps"},
					aiConfigMapName,
					errors.New("existing ConfigMap is not managed by the AI configuration writer"),
				),
			)
		}
		if !configMap.DeletionTimestamp.IsZero() {
			return fmt.Errorf("AI configuration ConfigMap %q is terminating", key.String())
		}
		if _, exists := configMap.BinaryData[aiConfigDataKey]; exists {
			return fmt.Errorf("AI configuration ConfigMap %q has binaryData occupying %q", key.String(), aiConfigDataKey)
		}
		if configMap.Data[aiConfigDataKey] == string(data) {
			return nil
		}
		if configMap.Immutable != nil && *configMap.Immutable {
			return fmt.Errorf("AI configuration ConfigMap %q is immutable", key.String())
		}
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}
	configMap.Data[aiConfigDataKey] = string(data)

	// 原生校验按 data 与 binaryData 的值字节总量计算，不只计算所属 key。
	if size := configMapDataSize(&configMap); size > corev1.MaxSecretSize {
		return fmt.Errorf("AI configuration ConfigMap %q data size %d exceeds %d bytes", key.String(), size, corev1.MaxSecretSize)
	}

	if create {
		if err := w.client.Create(ctx, &configMap); err != nil {
			return fmt.Errorf("create AI configuration ConfigMap %q: %w", key.String(), err)
		}

		return nil
	}

	if err := w.client.Update(ctx, &configMap); err != nil {
		return fmt.Errorf("update AI configuration ConfigMap %q: %w", key.String(), err)
	}

	return nil
}

func configMapDataSize(configMap *corev1.ConfigMap) int64 {
	var size int64
	for _, value := range configMap.Data {
		size += int64(len(value))
	}
	for _, value := range configMap.BinaryData {
		size += int64(len(value))
	}

	return size
}
