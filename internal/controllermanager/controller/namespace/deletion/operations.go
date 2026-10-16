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

// 来源：Kubernetes v1.37.0 namespaced_resources_deleter.go 的操作支持缓存。
// 本地适配：在每轮运行的 Discovery 结果中记录不支持的操作，不在构造时联网。
package deletion

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

type operation string

const (
	operationDeleteCollection operation = "deletecollection"
	operationList             operation = "list"
)

type operationKey struct {
	operation operation
	gvr       schema.GroupVersionResource
}

// operationNotSupportedCache 沿用官方互斥保护，供不同空间的删除调用共享。
// 与官方相同，不支持标记在实例生命周期内保留；不承诺 API 动词动态升级后的重新探测。
type operationNotSupportedCache struct {
	lock        sync.RWMutex
	unsupported map[operationKey]bool
}

func (o *operationNotSupportedCache) record(resources []*metav1.APIResourceList) {
	for _, list := range resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			// GroupVersionResources 会汇总解析错误并阻止 finalize，此处不重复处理。
			continue
		}

		for _, resource := range list.APIResources {
			gvr := gv.WithResource(resource.Name)
			verbs := sets.New[string](resource.Verbs...)
			for _, op := range []operation{operationList, operationDeleteCollection} {
				if !verbs.Has(string(op)) {
					o.setNotSupported(operationKey{operation: op, gvr: gvr})
				}
			}
		}
	}
}

func (o *operationNotSupportedCache) isSupported(key operationKey) bool {
	o.lock.RLock()
	defer o.lock.RUnlock()

	return !o.unsupported[key]
}

func (o *operationNotSupportedCache) setNotSupported(key operationKey) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.unsupported[key] = true
}
