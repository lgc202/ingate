package envoy

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"maps"
	"slices"

	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

// newSnapshot 在安装前建立名称唯一、引用闭合的候选，并生成原生 Delta 资源版本表。
// 构造后不再修改消息和版本表；即使先沿用 SotW，也可安全共享未变化的资源。
func newSnapshot(resources resourceSet) (*cachev3.Snapshot, error) {
	// go-control-plane 检查 LDS→RDS、CDS→EDS，但不检查 RDS→CDS。
	if err := checkClusterReferences(resources); err != nil {
		return nil, err
	}

	groups := []struct {
		typeURL resourcev3.Type
		items   []cachetypes.Resource
	}{
		{typeURL: resourcev3.ClusterType, items: make([]cachetypes.Resource, 0, len(resources.clusters))},
		{typeURL: resourcev3.EndpointType, items: make([]cachetypes.Resource, 0, len(resources.endpoints))},
		{typeURL: resourcev3.ListenerType, items: make([]cachetypes.Resource, 0, len(resources.listeners))},
		{typeURL: resourcev3.RouteType, items: make([]cachetypes.Resource, 0, len(resources.routes))},
	}
	for _, resource := range resources.clusters {
		groups[0].items = append(groups[0].items, resource)
	}

	for _, resource := range resources.endpoints {
		groups[1].items = append(groups[1].items, resource)
	}

	for _, resource := range resources.listeners {
		groups[2].items = append(groups[2].items, resource)
	}

	for _, resource := range resources.routes {
		groups[3].items = append(groups[3].items, resource)
	}

	snapshot := &cachev3.Snapshot{}
	for _, group := range groups {
		// 只规范化独立资源的排列，不调整 VirtualHost、Route 等有优先级的列表。
		slices.SortFunc(group.items, func(a, b cachetypes.Resource) int {
			return cmp.Compare(cachev3.GetResourceName(a), cachev3.GetResourceName(b))
		})
		for i := 1; i < len(group.items); i++ {
			name := cachev3.GetResourceName(group.items[i])
			if name == cachev3.GetResourceName(group.items[i-1]) {
				return nil, fmt.Errorf("duplicate %s resource %q", group.typeURL, name)
			}
		}

		index := cachev3.GetResponseType(group.typeURL)
		snapshot.Resources[index] = cachev3.NewResources("", group.items)
	}

	if err := snapshot.Consistent(); err != nil {
		return nil, fmt.Errorf("check xDS references: %w", err)
	}

	// 复用 go-control-plane 的确定性序列化与内容哈希，不另定义资源版本协议。
	// 在 SetSnapshot 前构造，避免读者可见后再惰性写入版本表。
	if err := snapshot.ConstructVersionMap(); err != nil {
		return nil, fmt.Errorf("version xDS resources: %w", err)
	}

	for _, group := range groups {
		index := cachev3.GetResponseType(group.typeURL)
		snapshot.Resources[index].Version = resourceVersion(snapshot.GetVersionMap(group.typeURL))
	}

	return snapshot, nil
}

// reuseSnapshotResources 只替换私有候选中的资源指针，不修改上一份已发布快照。
// 名称属于类型内的标识；只有同类型、同名、同内容版本才可共享消息。
func reuseSnapshotResources(current cachev3.ResourceSnapshot, next *cachev3.Snapshot) {
	for _, typeURL := range []resourcev3.Type{
		resourcev3.ClusterType, resourcev3.EndpointType, resourcev3.ListenerType, resourcev3.RouteType,
	} {
		previous := current.GetResources(typeURL)
		versions := current.GetVersionMap(typeURL)
		group := &next.Resources[cachev3.GetResponseType(typeURL)]
		for name, item := range group.Items {
			if old := previous[name]; old != nil && versions[name] == next.GetVersionMap(typeURL)[name] {
				item.Resource = old
				group.Items[name] = item
			}
		}
	}
}

func sameVersions(current cachev3.ResourceSnapshot, next *cachev3.Snapshot) bool {
	for _, typeURL := range []resourcev3.Type{
		resourcev3.ClusterType, resourcev3.EndpointType, resourcev3.ListenerType, resourcev3.RouteType,
	} {
		if current.GetVersion(typeURL) != next.GetVersion(typeURL) {
			return false
		}
	}

	return true
}

func checkClusterReferences(resources resourceSet) error {
	clusters := make(map[string]bool, len(resources.clusters))
	for _, cluster := range resources.clusters {
		clusters[cluster.Name] = true
	}

	for _, routeConfig := range resources.routes {
		for _, host := range routeConfig.VirtualHosts {
			for _, route := range host.Routes {
				// 直返动作不引用 Cluster；只校验转发动作的后端。
				if route.GetRoute() == nil {
					continue
				}

				name := route.GetRoute().GetCluster()
				if !clusters[name] {
					return fmt.Errorf("route %q references missing cluster %q", route.Name, name)
				}
			}
		}
	}

	return nil
}

// resourceVersion 从名称和原生资源版本聚合 SotW 类型级版本，不重复序列化消息。
func resourceVersion(versions map[string]string) string {
	var data []byte
	for _, name := range slices.Sorted(maps.Keys(versions)) {
		version := versions[name]
		data = binary.AppendUvarint(data, uint64(len(name)))
		data = append(data, name...)
		data = binary.AppendUvarint(data, uint64(len(version)))
		data = append(data, version...)
	}

	return cachev3.HashResource(data)
}
