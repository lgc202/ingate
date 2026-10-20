// Package configtest 为请求链测试提供原生内存 ADS 服务和显式编译配置夹具。
// 仅测试导入本包；领域编译仍由 Controller 完成，不作为另一条生产转换链使用。
package configtest

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	deltav3 "github.com/envoyproxy/go-control-plane/pkg/server/delta/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// Source 通过真实 gRPC 传输驱动 Loader，使用上游 LinearCache 生成资源版本和差异。
type Source struct {
	Client discoveryv3.AggregatedDiscoveryServiceClient
	cache  *cachev3.LinearCache
	names  map[string]bool
}

type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	server deltav3.Server
}

// New 创建测试拥有的内存 ADS 服务；初始化门禁由控制面联合测试验证。
func New(t *testing.T) *Source {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	cache := cachev3.NewLinearCache(aidataplanev1.ResourceType)
	server := grpc.NewServer()
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(server, &adsServer{server: deltav3.NewServer(context.Background(), cache, nil)})
	var worker sync.WaitGroup
	worker.Go(func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Serve(test ADS) error = %v", err)
		}
	})
	t.Cleanup(func() { server.Stop(); worker.Wait() })
	connection, err := grpc.NewClient("passthrough:///test-ads",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatalf("NewClient(test ADS) error = %v", err)
	}

	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("Close(test ADS connection) error = %v", err)
		}
	})

	return &Source{Client: discoveryv3.NewAggregatedDiscoveryServiceClient(connection), cache: cache, names: make(map[string]bool)}
}

// Publish 把已有请求测试的强类型编译夹具转换为资源；不会读取或调用 Kubernetes API。
func (s *Source) Publish(t *testing.T, config aidataplane.Config) {
	t.Helper()
	resources := Resources(t, config)
	updates := make(map[string]types.Resource, len(resources))
	names := make(map[string]bool, len(resources))
	for _, resource := range resources {
		updates[resource.Name] = resource
		names[resource.Name] = true
	}

	var removals []string
	for name := range s.names {
		if !names[name] {
			removals = append(removals, name)
		}
	}

	if err := s.cache.UpdateResources(updates, removals); err != nil {
		t.Fatalf("UpdateResources(test ADS) error = %v", err)
	}

	s.names = names
}

// Resources 保留夹具的模型精确匹配、拒绝语义和受保护出口；不作为生产编译入口。
func Resources(t *testing.T, config aidataplane.Config) []*aidataplanev1.Resource {
	t.Helper()
	networks := make(map[string]aidataplane.NetworkBackend)
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
	}

	backends := make(map[string]aidataplane.Backend)
	for _, backend := range config.Backends {
		backends[backend.Name] = backend
	}

	var resources []*aidataplanev1.Resource
	for _, listener := range config.Listeners {
		for _, host := range listener.Hosts {
			ref := &aidataplanev1.EntryReference{Port: listener.Port, HostName: host.Name}
			resources = append(resources, &aidataplanev1.Resource{
				Name:    fmt.Sprintf("entry/%d/%s", listener.Port, host.Name),
				Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{Port: listener.Port, HostName: host.Name, Hostname: *host.Hostname}},
			})
			for index, route := range host.ModelRoutes {
				origin, ruleIndex := modelOrigin(t, route.Name)
				rule := &aidataplanev1.ModelRule{
					EntryRefs: []*aidataplanev1.EntryReference{ref}, Origin: origin, RuleIndex: ruleIndex,
					Matches: []*aidataplanev1.ModelMatch{{Condition: &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}}},
					Outcome: &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}},
				}
				if len(route.Headers) != 0 {
					rule.Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{Exact: route.Headers[0].Value}
				}

				if backend := backends[*route.BackendName]; *route.BackendName != "" {
					target, err := aidataplane.ModelTargetName(backend, networks[backend.NetworkBackendName])
					if err != nil {
						t.Fatalf("ModelTargetName(test binding) error = %v", err)
					}

					rule.Outcome = &aidataplanev1.ModelRule_Target{Target: &aidataplanev1.ModelTarget{
						BindingName:       backend.Name,
						Schema:            &aidataplanev1.APISchema{Name: aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI, Prefix: "/" + strings.TrimPrefix(backend.Schema.Prefix, "/")},
						ModelNameOverride: *backend.ModelNameOverride, NetworkBackendName: backend.NetworkBackendName, TargetName: target,
					}}
				}

				resources = append(resources, &aidataplanev1.Resource{Name: fmt.Sprintf("model/%d/%s/%d", listener.Port, host.Name, index), Payload: &aidataplanev1.Resource_ModelRule{ModelRule: rule}})
			}

			for index, route := range host.MCPRoutes {
				parts := strings.SplitN(route.Name, "/", 2)
				compiled := &aidataplanev1.MCPRoute{
					EntryRefs: []*aidataplanev1.EntryReference{ref}, Path: route.Path,
					Origin:  &aidataplanev1.Origin{Namespace: parts[0], Name: parts[1], CreationTime: timestamppb.New(time.Unix(int64(index), 0))},
					Outcome: &aidataplanev1.MCPRoute_Unavailable{Unavailable: &emptypb.Empty{}},
				}
				if *route.BackendName != "" {
					target, err := aidataplane.MCPTargetName(networks[*route.BackendName])
					if err != nil {
						t.Fatalf("MCPTargetName(test binding) error = %v", err)
					}

					compiled.Outcome = &aidataplanev1.MCPRoute_Target{Target: &aidataplanev1.MCPTarget{
						NetworkBackendName: *route.BackendName, BackendPath: route.BackendPath, TargetName: target,
					}}
				}

				resources = append(resources, &aidataplanev1.Resource{Name: fmt.Sprintf("mcp/%d/%s/%d", listener.Port, host.Name, index), Payload: &aidataplanev1.Resource_McpRoute{McpRoute: compiled}})
			}
		}
	}

	return resources
}

func (s *adsServer) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return s.server.DeltaStreamHandler(stream, resourcev3.AnyType)
}

func modelOrigin(t *testing.T, name string) (*aidataplanev1.Origin, uint32) {
	t.Helper()
	parts := strings.Split(name, "/")
	if len(parts) != 6 {
		t.Fatalf("modelOrigin(%q) requires a namespace/name/rule/N/match/N fixture", name)
	}

	index, err := strconv.ParseUint(parts[3], 10, 32)
	if err != nil {
		t.Fatalf("modelOrigin(%q) rule index error = %v", name, err)
	}

	return &aidataplanev1.Origin{Namespace: parts[0], Name: parts[1], CreationTime: timestamppb.New(time.Unix(1, 0))}, uint32(index)
}
