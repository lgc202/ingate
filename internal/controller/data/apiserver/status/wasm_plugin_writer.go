package status

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (w *Writer) updateWasmPlugin(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.WasmPlugins().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := slices.Clone(resource.Status.Conditions)
		// 插件安装状态只表示制品能否被 Controller 拉取和校验，不依赖某次 Envoy 配置发布
		meta.RemoveStatusCondition(&conditions, string(gatewayv1.ConditionResolvedRefs))
		meta.RemoveStatusCondition(&conditions, string(gatewayv1.ConditionProgrammed))
		if compile != nil {
			meta.SetStatusCondition(&conditions, newCondition(
				gatewayv1.ConditionAccepted,
				source.Generation,
				compile.accepted,
			))
		}
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.WasmPlugins().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update WasmPlugin %q conditions: %w", source.Name, err)
	}
	return nil
}
