/*
Copyright 2019 The Kubernetes Authors.

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

// 来源：Kubernetes v1.37.0 pkg/controller/namespace/deletion/status_condition_utils.go。
// 本地适配：不导出未被消费的接口；按调用顺序组织方法；保留官方 Namespace Conditions。
package deletion

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

type namespaceConditionUpdater struct {
	newConditions       []v1.NamespaceCondition
	deleteContentErrors []error
}

var (
	// 官方删除 Conditions 及成功消息为固定协议值；初始化后不修改这些表。
	conditionTypes = []v1.NamespaceConditionType{
		v1.NamespaceDeletionDiscoveryFailure,
		v1.NamespaceDeletionGVParsingFailure,
		v1.NamespaceDeletionContentFailure,
		v1.NamespaceContentRemaining,
		v1.NamespaceFinalizersRemaining,
	}
	okMessages = map[v1.NamespaceConditionType]string{
		v1.NamespaceDeletionDiscoveryFailure: "All resources successfully discovered",
		v1.NamespaceDeletionGVParsingFailure: "All legacy kube types successfully parsed",
		v1.NamespaceDeletionContentFailure:   "All content successfully deleted, may be waiting on finalization",
		v1.NamespaceContentRemaining:         "All content successfully removed",
		v1.NamespaceFinalizersRemaining:      "All content-preserving finalizers finished",
	}
	okReasons = map[v1.NamespaceConditionType]string{
		v1.NamespaceDeletionDiscoveryFailure: "ResourcesDiscovered",
		v1.NamespaceDeletionGVParsingFailure: "ParsedGroupVersions",
		v1.NamespaceDeletionContentFailure:   "ContentDeleted",
		v1.NamespaceContentRemaining:         "ContentRemoved",
		v1.NamespaceFinalizersRemaining:      "ContentHasNoFinalizers",
	}
)

// ProcessDiscoverResourcesErr 保留发现失败的原因；已发现类型仍可继续清理。
func (u *namespaceConditionUpdater) ProcessDiscoverResourcesErr(err error) {
	message := err.Error()
	if discoveryErr, ok := errors.AsType[*discovery.ErrGroupDiscoveryFailed](err); ok {
		message = fmt.Sprintf("Discovery failed for some groups, %d failing: %v", len(discoveryErr.Groups), err)
	}

	condition := v1.NamespaceCondition{
		Type:               v1.NamespaceDeletionDiscoveryFailure,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "DiscoveryFailed",
		Message:            message,
	}
	u.newConditions = append(u.newConditions, condition)
}

// ProcessGroupVersionErr 记录发现结果的版本解析失败，阻止将未知类型视为已清空。
func (u *namespaceConditionUpdater) ProcessGroupVersionErr(err error) {
	condition := v1.NamespaceCondition{
		Type:               v1.NamespaceDeletionGVParsingFailure,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "GroupVersionParsingFailed",
		Message:            err.Error(),
	}
	u.newConditions = append(u.newConditions, condition)
}

// ProcessDeleteContentErr 收集各资源类型的清理错误，随后统一更新状态。
func (u *namespaceConditionUpdater) ProcessDeleteContentErr(err error) {
	u.deleteContentErrors = append(u.deleteContentErrors, err)
}

// ProcessContentTotals 分别记录资源残留和对象 finalizer，供调用方定位阻塞清理的内容。
func (u *namespaceConditionUpdater) ProcessContentTotals(contentTotals allGVRDeletionMetadata) {
	if len(contentTotals.gvrToNumRemaining) != 0 {
		var remainingResources []string
		for gvr, numRemaining := range contentTotals.gvrToNumRemaining {
			if numRemaining == 0 {
				continue
			}

			message := fmt.Sprintf("%s.%s has %d resource instances", gvr.Resource, gvr.Group, numRemaining)
			remainingResources = append(remainingResources, message)
		}

		// 固定消息顺序，避免 map 遍历顺序触发无意义的状态写入。
		sort.Strings(remainingResources)
		u.newConditions = append(u.newConditions, v1.NamespaceCondition{
			Type:               v1.NamespaceContentRemaining,
			Status:             v1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "SomeResourcesRemain",
			Message:            fmt.Sprintf("Some resources are remaining: %s", strings.Join(remainingResources, ", ")),
		})
	}

	if len(contentTotals.finalizersToNumRemaining) != 0 {
		var remainingByFinalizer []string
		for finalizer, numRemaining := range contentTotals.finalizersToNumRemaining {
			if numRemaining == 0 {
				continue
			}

			message := fmt.Sprintf("%s in %d resource instances", finalizer, numRemaining)
			remainingByFinalizer = append(remainingByFinalizer, message)
		}

		// 固定消息顺序，避免 map 遍历顺序触发无意义的状态写入。
		sort.Strings(remainingByFinalizer)
		u.newConditions = append(u.newConditions, v1.NamespaceCondition{
			Type:               v1.NamespaceFinalizersRemaining,
			Status:             v1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "SomeFinalizersRemain",
			Message:            fmt.Sprintf("Some content in the namespace has finalizers remaining: %s", strings.Join(remainingByFinalizer, ", ")),
		})
	}
}

// Update 合并本轮结果并清除已恢复的失败标记；不修改其它控制器维护的 Conditions。
func (u *namespaceConditionUpdater) Update(namespace *v1.Namespace) bool {
	if c := getCondition(u.newConditions, v1.NamespaceDeletionContentFailure); c == nil {
		if c := makeDeleteContentCondition(u.deleteContentErrors); c != nil {
			u.newConditions = append(u.newConditions, *c)
		}
	}

	return updateConditions(&namespace.Status, u.newConditions)
}

func makeDeleteContentCondition(errs []error) *v1.NamespaceCondition {
	if len(errs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	sort.Strings(messages)

	return &v1.NamespaceCondition{
		Type:               v1.NamespaceDeletionContentFailure,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ContentDeletionFailed",
		Message:            fmt.Sprintf("Failed to delete all resource types, %d remaining: %v", len(errs), strings.Join(messages, ", ")),
	}
}

func updateConditions(status *v1.NamespaceStatus, newConditions []v1.NamespaceCondition) bool {
	var hasChanged bool
	for _, conditionType := range conditionTypes {
		newCondition := getCondition(newConditions, conditionType)
		// 本轮没有该失败类型时使用官方成功值，清除前一轮的失败状态。
		if newCondition == nil {
			newCondition = newSuccessfulCondition(conditionType)
		}

		oldCondition := getCondition(status.Conditions, conditionType)

		// 首次出现的类型追加到列表，已有自定义 Conditions 保持原样。
		if oldCondition == nil {
			status.Conditions = append(status.Conditions, *newCondition)
			hasChanged = true
			continue
		}

		if oldCondition.Status == newCondition.Status &&
			oldCondition.Message == newCondition.Message &&
			oldCondition.Reason == newCondition.Reason {
			continue
		}

		// 仅状态真假变化才更新时间；原因或消息变化不代表重新进入该状态。
		if oldCondition.Status != newCondition.Status {
			oldCondition.LastTransitionTime = metav1.Now()
		}

		oldCondition.Type = newCondition.Type
		oldCondition.Status = newCondition.Status
		oldCondition.Reason = newCondition.Reason
		oldCondition.Message = newCondition.Message
		hasChanged = true
	}

	return hasChanged
}

func newSuccessfulCondition(conditionType v1.NamespaceConditionType) *v1.NamespaceCondition {
	return &v1.NamespaceCondition{
		Type:               conditionType,
		Status:             v1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             okReasons[conditionType],
		Message:            okMessages[conditionType],
	}
}

func getCondition(conditions []v1.NamespaceCondition, conditionType v1.NamespaceConditionType) *v1.NamespaceCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}
