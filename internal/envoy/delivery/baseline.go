package delivery

import (
	"fmt"

	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

// BaselineVersion 是没有可服务配置时写入四类 xDS 资源的固定版本
const BaselineVersion = "ingate/baseline"

func newBaselineSnapshot() (*cachev3.Snapshot, error) {
	snapshot, err := cachev3.NewSnapshot(BaselineVersion, map[resourcev3.Type][]cachetypes.Resource{
		resourcev3.ListenerType: {},
		resourcev3.RouteType:    {},
		resourcev3.ClusterType:  {},
		resourcev3.EndpointType: {},
	})
	if err != nil {
		return nil, fmt.Errorf("create baseline snapshot: %w", err)
	}
	if err := snapshot.Consistent(); err != nil {
		return nil, fmt.Errorf("check baseline snapshot consistency: %w", err)
	}
	for _, typeURL := range dynamicTypeURLs() {
		if snapshot.GetVersion(typeURL) == "" {
			return nil, fmt.Errorf("baseline snapshot has an empty version for %q", typeURL)
		}
		if len(snapshot.GetResources(typeURL)) != 0 {
			return nil, fmt.Errorf("baseline snapshot has resources for %q", typeURL)
		}
	}
	return snapshot, nil
}
