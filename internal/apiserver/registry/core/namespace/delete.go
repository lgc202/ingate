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

// 来源：Kubernetes v1.37.0 pkg/registry/core/namespace/storage/storage.go。
// 本地适配：复制 DeleteOptions；保留 finalizer 顺序；拆分删除阶段和传播策略处理。
package namespace

import (
	"context"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	storageerr "k8s.io/apiserver/pkg/storage/errors"
	"k8s.io/apiserver/pkg/util/dryrun"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// Delete 首次标记 Terminating；后续必须等待 spec 和 metadata 两类 finalizer。
func (r *REST) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	obj, err := r.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	namespace := obj.(*coreapi.Namespace)
	if options == nil {
		options = metav1.NewDeleteOptions(0)
	} else {
		options = options.DeepCopy()
	}
	if options.Preconditions == nil {
		options.Preconditions = &metav1.Preconditions{}
	}
	resource := coreapi.SchemeGroupVersion.WithResource("namespaces").GroupResource()
	if options.Preconditions.UID == nil {
		options.Preconditions.UID = &namespace.UID
	} else if *options.Preconditions.UID != namespace.UID {
		return nil, false, apierrors.NewConflict(resource, name, errors.New("UID precondition does not match"))
	}
	if options.Preconditions.ResourceVersion != nil && *options.Preconditions.ResourceVersion != namespace.ResourceVersion {
		return nil, false, apierrors.NewConflict(resource, name, errors.New("resourceVersion precondition does not match"))
	}

	if namespace.DeletionTimestamp.IsZero() {
		terminating, err := r.beginTermination(ctx, name, deleteValidation, options)
		return terminating, false, err
	}
	if len(namespace.Spec.Finalizers) != 0 {
		return namespace, false, nil
	}
	return r.store.Delete(ctx, name, deleteValidation, options)
}

func (r *REST) beginTermination(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, error) {
	key, err := r.store.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}

	preconditions := storage.Preconditions{
		UID:             options.Preconditions.UID,
		ResourceVersion: options.Preconditions.ResourceVersion,
	}
	update := storage.SimpleUpdate(func(existing runtime.Object) (runtime.Object, error) {
		namespace, ok := existing.(*coreapi.Namespace)
		if !ok {
			return nil, fmt.Errorf("expected *core.Namespace, got %T", existing)
		}
		if deleteValidation != nil {
			if err := deleteValidation(ctx, namespace); err != nil {
				return nil, err
			}
		}

		if namespace.DeletionTimestamp.IsZero() {
			now := metav1.Now()
			namespace.DeletionTimestamp = &now
		}
		namespace.Status.Phase = corev1.NamespaceTerminating
		setPropagationFinalizers(namespace, options)
		return namespace, nil
	})
	out := r.store.NewFunc()
	err = r.store.Storage.GuaranteedUpdate(ctx, key, out, false, &preconditions, update, dryrun.IsDryRun(options.DryRun), nil)
	if err != nil {
		resource := coreapi.SchemeGroupVersion.WithResource("namespaces").GroupResource()
		err = storageerr.InterpretGetError(err, resource, name)
		err = storageerr.InterpretUpdateError(err, resource, name)
		var statusError *apierrors.StatusError
		if !errors.As(err, &statusError) {
			err = apierrors.NewInternalError(err)
		}
		return nil, err
	}
	return out, nil
}

func shouldDeleteDuringUpdate(ctx context.Context, key string, obj, existing runtime.Object) bool {
	namespace, ok := obj.(*coreapi.Namespace)
	if !ok {
		utilruntime.HandleErrorWithContext(ctx, fmt.Errorf("unexpected type %T", obj), "Cannot finalize Namespace")
		return false
	}
	return len(namespace.Spec.Finalizers) == 0 && genericregistry.ShouldDeleteDuringUpdate(ctx, key, obj, existing)
}

func setPropagationFinalizers(namespace *coreapi.Namespace, options *metav1.DeleteOptions) {
	haveOrphan := slices.Contains(namespace.Finalizers, metav1.FinalizerOrphanDependents)
	haveForeground := slices.Contains(namespace.Finalizers, metav1.FinalizerDeleteDependents)
	wantOrphan := shouldHaveOrphanFinalizer(options, haveOrphan)
	wantForeground := shouldHaveDeleteDependentsFinalizer(options, haveForeground)
	if haveOrphan == wantOrphan && haveForeground == wantForeground {
		return
	}

	finalizers := make([]string, 0, len(namespace.Finalizers)+2)
	for _, finalizer := range namespace.Finalizers {
		if finalizer == metav1.FinalizerOrphanDependents && !wantOrphan {
			continue
		}
		if finalizer == metav1.FinalizerDeleteDependents && !wantForeground {
			continue
		}
		finalizers = append(finalizers, finalizer)
	}
	if wantOrphan && !haveOrphan {
		finalizers = append(finalizers, metav1.FinalizerOrphanDependents)
	}
	if wantForeground && !haveForeground {
		finalizers = append(finalizers, metav1.FinalizerDeleteDependents)
	}
	namespace.Finalizers = finalizers
}

func shouldHaveOrphanFinalizer(options *metav1.DeleteOptions, haveOrphan bool) bool {
	//nolint:staticcheck // 标准 DeleteOptions 仍公开 OrphanDependents；保留其优先级。
	if options.OrphanDependents != nil {
		return *options.OrphanDependents
	}
	if options.PropagationPolicy != nil {
		return *options.PropagationPolicy == metav1.DeletePropagationOrphan
	}
	return haveOrphan
}

func shouldHaveDeleteDependentsFinalizer(options *metav1.DeleteOptions, haveForeground bool) bool {
	//nolint:staticcheck // 标准 DeleteOptions 仍公开 OrphanDependents；保留其优先级。
	if options.OrphanDependents != nil {
		return !*options.OrphanDependents
	}
	if options.PropagationPolicy != nil {
		return *options.PropagationPolicy == metav1.DeletePropagationForeground
	}
	return haveForeground
}
