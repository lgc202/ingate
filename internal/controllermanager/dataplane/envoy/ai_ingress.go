package envoy

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

const aiIngressName = "ai/ingress"

type modelEntryKey struct {
	routeName string
	hostName  string
}

func newAIIngressCluster(socket string) (*clusterv3.Cluster, error) {
	cluster := &clusterv3.Cluster{
		Name:                 aiIngressName,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STATIC},
		ConnectTimeout:       durationpb.New(5 * time.Second),
		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: aiIngressName,
			Endpoints: []*endpointv3.LocalityLbEndpoints{{LbEndpoints: []*endpointv3.LbEndpoint{{
				HostIdentifier: &endpointv3.LbEndpoint_Endpoint{Endpoint: &endpointv3.Endpoint{
					Address: &corev3.Address{Address: &corev3.Address_Pipe{Pipe: &corev3.Pipe{Path: socket}}},
				}},
			}}}},
		},
	}
	if err := cluster.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate AI ingress cluster: %w", err)
	}

	return cluster, nil
}

func newAIIngressRoute(port int32, hostName, operationPath string) *routev3.Route {
	return &routev3.Route{
		Name: aiIngressName + "/" + hostName + operationPath,
		Match: &routev3.RouteMatch{
			PathSpecifier: &routev3.RouteMatch_Path{Path: operationPath},
			Headers:       []*routev3.HeaderMatcher{exactAIHeader(":method", "POST")},
		},
		Action: &routev3.Route_Route{Route: &routev3.RouteAction{
			ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: aiIngressName},
			Timeout:          durationpb.New(0),
		}},
		RequestHeadersToAdd: []*corev3.HeaderValueOption{
			{
				Header:       &corev3.HeaderValue{Key: aidataplane.ListenerPortHeader, Value: strconv.Itoa(int(port))},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
			{
				Header:       &corev3.HeaderValue{Key: aidataplane.HostNameHeader, Value: hostName},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
		},
		RequestHeadersToRemove: []string{aidataplane.TargetHeader},
	}
}

// mergeAIIngress 按 Listener/Host 合入模型入口。POST 精确匹配排在普通 HTTPRoute
// 之前，其他请求保持原匹配顺序；只有 MCPRoute 的 Host 不会增加模型入口。
// HTTP 与 AI 视图独立到达，尚未出现的公共 Host 不凭 AI 配置单独开放。
// 不修改传入 proto，防止更新污染已发布的 Snapshot 或积累重复路由。
func mergeAIIngress(
	routes []*routev3.RouteConfiguration,
	models map[modelEntryKey][]*routev3.Route,
) []*routev3.RouteConfiguration {
	merged := make([]*routev3.RouteConfiguration, 0, len(routes))
	for _, config := range routes {
		clone := proto.Clone(config).(*routev3.RouteConfiguration)
		for _, host := range clone.VirtualHosts {
			key := modelEntryKey{routeName: config.Name, hostName: host.Name}
			if entries := models[key]; len(entries) != 0 {
				host.Routes = slices.Concat(entries, host.Routes)
			}
		}

		merged = append(merged, clone)
	}

	return merged
}
