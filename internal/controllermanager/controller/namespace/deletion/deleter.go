/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// 来源：Kubernetes v1.37.0 pkg/controller/namespace/deletion/namespaced_resources_deleter.go。
// 本地适配：只处理 Ingate 资源；不含 Pod 终止与排序；构造不访问 API；冲突重试有界。
// Discovery 使用 client-go 的带 Context 方法，保持协调请求的取消传播。

// Package deletion 根据 Discovery 清理空间内容，确认清空后移除 Namespace 的 kubernetes finalizer。
package deletion

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	v1clientset "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/util/retry"
)

// finalizerEstimateSeconds 沿用官方等待对象 finalizer 的估计值，单位为秒。
const finalizerEstimateSeconds = int64(15)

// Deleter 执行官方 Namespace 删除流程，不持有工作队列或运行协程。
// Namespace 和 Metadata 客户端以及 Discovery 函数必须指向同一个 Ingate API Server。
type Deleter struct {
	namespaceClient     v1clientset.NamespaceInterface
	metadataClient      metadata.Interface
	opCache             *operationNotSupportedCache
	discoverResourcesFn func(context.Context) ([]*metav1.APIResourceList, error)
	finalizerToken      v1.FinalizerName
}

type updateNamespaceFunc func(context.Context, *v1.Namespace) (*v1.Namespace, error)

// ResourcesRemainingError 表示内容尚在等待对象 finalizer 完成，不应释放 Namespace finalizer。
type ResourcesRemainingError struct {
	// Estimate 沿用官方的秒数估计，供调用方安排下一次检查，不是清理完成的承诺。
	Estimate int64
}

type gvrDeletionMetadata struct {
	finalizerEstimateSeconds int64
	numRemaining             int
	finalizersToNumRemaining map[string]int
}

type allGVRDeletionMetadata struct {
	gvrToNumRemaining        map[schema.GroupVersionResource]int
	finalizersToNumRemaining map[string]int
}

// New 保存原生客户端和发现入口，不发送请求；调用方负责调用 Delete 和调度重试。
func New(
	namespaceClient v1clientset.NamespaceInterface,
	metadataClient metadata.Interface,
	discoverResourcesFn func(context.Context) ([]*metav1.APIResourceList, error),
) *Deleter {
	return &Deleter{
		namespaceClient:     namespaceClient,
		metadataClient:      metadataClient,
		discoverResourcesFn: discoverResourcesFn,
		opCache:             &operationNotSupportedCache{unsupported: make(map[operationKey]bool)},
		finalizerToken:      v1.FinalizerKubernetes,
	}
}

// Delete 忽略不存在或尚未删除的 Namespace；清理失败或存在残留时保留 finalizer。
// 状态更新和 finalize 通过原生子资源 API 完成，不直接删除存储记录。
func (d *Deleter) Delete(ctx context.Context, name string) error {
	namespace, err := d.namespaceClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get namespace %q: %w", name, err)
	}
	if namespace.DeletionTimestamp == nil {
		return nil
	}

	namespace, err = d.updateNamespaceWithRetry(ctx, namespace, d.updateNamespaceStatus)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("mark namespace %q as Terminating: %w", name, err)
	}

	if namespace.DeletionTimestamp.IsZero() {
		return nil
	}

	if len(namespace.Spec.Finalizers) == 0 {
		return nil
	}

	estimate, err := d.deleteAllContent(ctx, namespace)
	if err != nil {
		return err
	}
	if estimate > 0 {
		return &ResourcesRemainingError{Estimate: estimate}
	}

	_, err = d.updateNamespaceWithRetry(ctx, namespace, d.finalizeNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("finalize namespace %q: %w", name, err)
	}
	return nil
}

// updateNamespaceWithRetry 使用官方有界退避，耗尽后交还调用方，不在删除内核中无限重试。
// 每次重读都检查原 UID，避免对同名重建空间执行状态更新或 finalize。
func (d *Deleter) updateNamespaceWithRetry(
	ctx context.Context,
	namespace *v1.Namespace,
	update updateNamespaceFunc,
) (*v1.Namespace, error) {
	var result *v1.Namespace
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := d.namespaceClient.Get(ctx, namespace.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("re-read namespace: %w", err)
		}
		if current.UID != namespace.UID {
			return fmt.Errorf("namespace UID changed from %q to %q during deletion", namespace.UID, current.UID)
		}
		result, err = update(ctx, current)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// updateNamespaceStatus 只通过 status 子资源把正在删除的空间标为 Terminating。
func (d *Deleter) updateNamespaceStatus(ctx context.Context, namespace *v1.Namespace) (*v1.Namespace, error) {
	if namespace.DeletionTimestamp.IsZero() || namespace.Status.Phase == v1.NamespaceTerminating {
		return namespace, nil
	}

	updated := namespace.DeepCopy()
	updated.Status.Phase = v1.NamespaceTerminating
	return d.namespaceClient.UpdateStatus(ctx, updated, metav1.UpdateOptions{})
}

// deleteAllContent 按 Discovery 的可删除资源逐类清理；部分发现失败仍清理已知类型，但返回错误阻止 finalize。
func (d *Deleter) deleteAllContent(ctx context.Context, namespace *v1.Namespace) (int64, error) {
	var (
		errs             []error
		conditionUpdater namespaceConditionUpdater
		estimate         int64
	)

	resources, err := d.discoverResourcesFn(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("discover resources for namespace %q: %w", namespace.Name, err))
		conditionUpdater.ProcessDiscoverResourcesErr(err)
	}
	d.opCache.record(resources)

	// 清理入口只接纳空间级主资源；即使发现结果混入集群级对象或子资源，也不向它们发起删除。
	namespacedResources := discovery.FilteredBy(discovery.ResourcePredicateFunc(
		func(_ string, resource *metav1.APIResource) bool {
			return resource.Namespaced && !strings.Contains(resource.Name, "/")
		},
	), resources)
	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete"}}, namespacedResources)
	groupVersionResources, err := discovery.GroupVersionResources(deletableResources)
	if err != nil {
		errs = append(errs, fmt.Errorf("parse discovered resources for namespace %q: %w", namespace.Name, err))
		conditionUpdater.ProcessGroupVersionErr(err)
	}

	numRemainingTotals := allGVRDeletionMetadata{
		gvrToNumRemaining:        make(map[schema.GroupVersionResource]int),
		finalizersToNumRemaining: make(map[string]int),
	}
	for gvr := range groupVersionResources {
		content, err := d.deleteAllContentForGroupVersionResource(ctx, gvr, namespace.Name)
		if err != nil {
			err = fmt.Errorf("delete %s in namespace %q: %w", gvr, namespace.Name, err)
			errs = append(errs, err)
			conditionUpdater.ProcessDeleteContentErr(err)
		}
		estimate = max(estimate, content.finalizerEstimateSeconds)
		if content.numRemaining > 0 {
			numRemainingTotals.gvrToNumRemaining[gvr] = content.numRemaining
			for finalizer, numRemaining := range content.finalizersToNumRemaining {
				if numRemaining == 0 {
					continue
				}
				numRemainingTotals.finalizersToNumRemaining[finalizer] += numRemaining
			}
		}
	}

	conditionUpdater.ProcessContentTotals(numRemainingTotals)

	if conditionUpdater.Update(namespace) {
		if _, err := d.namespaceClient.UpdateStatus(ctx, namespace, metav1.UpdateOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("update namespace %q deletion conditions: %w", namespace.Name, err))
		}
	}

	return estimate, utilerrors.NewAggregate(errs)
}

// deleteAllContentForGroupVersionResource 优先批量删除，退回逐个删除，然后重新 List 验证残留。
// 无 finalizer 的异常残留同时返回数量和错误，供调用方写入残留及失败 Conditions。
func (d *Deleter) deleteAllContentForGroupVersionResource(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
) (gvrDeletionMetadata, error) {
	deleteCollectionSupported, err := d.deleteCollection(ctx, gvr, namespace)
	if err != nil {
		return gvrDeletionMetadata{}, err
	}

	if !deleteCollectionSupported {
		err = d.deleteEachItem(ctx, gvr, namespace)
		if err != nil {
			return gvrDeletionMetadata{}, err
		}
	}

	remaining, listSupported, err := d.listCollection(ctx, gvr, namespace)
	if err != nil {
		return gvrDeletionMetadata{}, err
	}
	if !listSupported {
		return gvrDeletionMetadata{}, nil
	}
	if len(remaining.Items) == 0 {
		return gvrDeletionMetadata{}, nil
	}

	finalizersToNumRemaining := make(map[string]int)
	for _, item := range remaining.Items {
		for _, finalizer := range item.GetFinalizers() {
			finalizersToNumRemaining[finalizer]++
		}
	}

	if len(finalizersToNumRemaining) > 0 {
		return gvrDeletionMetadata{
			finalizerEstimateSeconds: finalizerEstimateSeconds,
			numRemaining:             len(remaining.Items),
			finalizersToNumRemaining: finalizersToNumRemaining,
		}, nil
	}

	return gvrDeletionMetadata{
		numRemaining: len(remaining.Items),
	}, fmt.Errorf("unexpected %d items remain without finalizers", len(remaining.Items))
}

// deleteCollection 使用 Background 传播，不添加 orphan finalizer；不支持批量删除时交给调用方回退。
func (d *Deleter) deleteCollection(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
) (bool, error) {
	key := operationKey{operation: operationDeleteCollection, gvr: gvr}
	if !d.opCache.isSupported(key) {
		return false, nil
	}

	background := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{PropagationPolicy: &background}
	err := d.metadataClient.Resource(gvr).Namespace(namespace).DeleteCollection(ctx, opts, metav1.ListOptions{})
	if err == nil {
		return true, nil
	}

	if errors.IsMethodNotSupported(err) || errors.IsNotFound(err) {
		return false, nil
	}

	return true, fmt.Errorf("delete collection: %w", err)
}

// deleteEachItem 沿用官方 List 后逐个 Delete 的回退路径，不跨 Namespace 清理。
func (d *Deleter) deleteEachItem(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
) error {
	partialList, listSupported, err := d.listCollection(ctx, gvr, namespace)
	if err != nil {
		return err
	}
	if !listSupported {
		return nil
	}

	for _, item := range partialList.Items {
		background := metav1.DeletePropagationBackground
		opts := metav1.DeleteOptions{PropagationPolicy: &background}
		err := d.metadataClient.Resource(gvr).Namespace(namespace).Delete(ctx, item.GetName(), opts)
		if err != nil && !errors.IsNotFound(err) && !errors.IsMethodNotSupported(err) {
			return fmt.Errorf("delete object %q: %w", item.GetName(), err)
		}
	}
	return nil
}

// listCollection 沿用官方对不支持 List 的资源处理；其它读取失败必须阻止 finalize。
func (d *Deleter) listCollection(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
) (*metav1.PartialObjectMetadataList, bool, error) {
	key := operationKey{operation: operationList, gvr: gvr}
	if !d.opCache.isSupported(key) {
		return nil, false, nil
	}

	partialList, err := d.metadataClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		return partialList, true, nil
	}

	if errors.IsMethodNotSupported(err) || errors.IsNotFound(err) {
		return nil, false, nil
	}

	return nil, true, fmt.Errorf("list collection: %w", err)
}

// finalizeNamespace 只移除本控制器拥有的 token，保留其它 Namespace finalizer。
func (d *Deleter) finalizeNamespace(ctx context.Context, namespace *v1.Namespace) (*v1.Namespace, error) {
	namespaceFinalize := v1.Namespace{
		ObjectMeta: namespace.ObjectMeta,
		Spec:       namespace.Spec,
	}
	finalizerSet := sets.New[string]()
	for i := range namespace.Spec.Finalizers {
		if namespace.Spec.Finalizers[i] != d.finalizerToken {
			finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
		}
	}
	namespaceFinalize.Spec.Finalizers = make([]v1.FinalizerName, 0, len(finalizerSet))
	for _, value := range sets.List(finalizerSet) {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, v1.FinalizerName(value))
	}

	namespace, err := d.namespaceClient.Finalize(ctx, &namespaceFinalize, metav1.UpdateOptions{})
	if errors.IsNotFound(err) {
		return namespace, nil
	}
	return namespace, err
}

// Error 描述内容仍未清空；调用方应使用 errors.AsType 区分正常等待与操作失败。
func (e *ResourcesRemainingError) Error() string {
	return fmt.Sprintf("some content remains in the namespace, estimate %d seconds before it is removed", e.Estimate)
}
