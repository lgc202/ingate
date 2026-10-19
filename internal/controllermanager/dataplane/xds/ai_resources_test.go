package xds

import (
	"context"
	"slices"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

func TestAIProxyDeltaADSUpdatesDeletesAndResumes(t *testing.T) {
	resources := aiTestResources()
	publisher := NewPublisher("", "")
	publishTestAIResources(t, publisher, resources)
	initialEnvoy := publishTestSnapshot(t, publisher, twoPortConfig())
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	firstCtx, closeFirst := context.WithCancel(ctx)
	defer closeFirst()
	stream, err := client.DeltaAggregatedResources(firstCtx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(AI Proxy initial) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"},
	})
	first := receiveTestDelta(t, stream)
	versions := make(map[string]string)
	if first.TypeUrl != aidataplanev1.ResourceType || len(first.Resources) != 3 || len(first.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(AI Proxy initial) = %v, want three AI Proxy resources without removals", first)
	}

	for _, resource := range first.Resources {
		var decoded aidataplanev1.Resource
		if err := resource.Resource.UnmarshalTo(&decoded); err != nil {
			t.Fatalf("UnmarshalTo(AI Proxy %q) error = %v", resource.Name, err)
		}

		data, err := cachev3.MarshalResource(&decoded)
		if err != nil {
			t.Fatalf("MarshalResource(AI Proxy %q) error = %v", resource.Name, err)
		}

		if decoded.Name != resource.Name || resource.Version != cachev3.HashResource(data) {
			t.Errorf("Delta ADS(AI Proxy %q) name/version = %q/%q, want equal envelope name and native content hash", resource.Name, decoded.Name, resource.Version)
		}

		versions[resource.Name] = resource.Version
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: aidataplanev1.ResourceType, ResponseNonce: first.Nonce})
	// 相等发布不能产生多余响应；下一次变化只能返回 rule/a。
	publishTestAIResources(t, publisher, resources)
	resources[1].GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{Exact: "changed-a"}
	publishTestAIResources(t, publisher, resources)

	update := receiveTestDelta(t, stream)
	if len(update.Resources) != 1 || update.Resources[0].Name != "model/team-a/a/rule/0" || len(update.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(AI Proxy rule update) = %v, want only rule/a", update)
	}

	if update.Resources[0].Version == versions[update.Resources[0].Name] {
		t.Error("Delta ADS(AI Proxy rule update) retained old version")
	}

	versions[update.Resources[0].Name] = update.Resources[0].Version
	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: aidataplanev1.ResourceType, ResponseNonce: update.Nonce})
	closeFirst()
	resources = []*aidataplanev1.Resource{resources[0], resources[2]}
	resources[1].GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{Exact: "changed-b"}
	publishTestAIResources(t, publisher, resources)
	resumed, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(AI Proxy resumed) error = %v", err)
	}

	sendTestDelta(t, resumed, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"}, InitialResourceVersions: versions,
	})
	catchup := receiveTestDelta(t, resumed)
	if len(catchup.Resources) != 1 || catchup.Resources[0].Name != "model/team-a/b/rule/0" ||
		!slices.Equal(catchup.RemovedResources, []string{"model/team-a/a/rule/0"}) {
		t.Fatalf("Delta ADS(AI Proxy resumed) = %v, want changed rule/b and removed rule/a", catchup)
	}

	delete(versions, "model/team-a/a/rule/0")
	versions[catchup.Resources[0].Name] = catchup.Resources[0].Version
	sendTestDelta(t, resumed, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: aidataplanev1.ResourceType, ResponseNonce: catchup.Nonce})

	// 使用全新缓存模拟控制面重建，不能因为安装顺序或本地计数变化重发相同资源。
	rebuilt := NewPublisher("", "")
	slices.Reverse(resources)
	publishTestAIResources(t, rebuilt, resources)
	rebuiltClient := newTestADSClient(t, rebuilt, logr.Discard())
	rebuiltStream, err := rebuiltClient.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(rebuilt cache) error = %v", err)
	}

	sendTestDelta(t, rebuiltStream, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"}, InitialResourceVersions: versions,
	})
	known := receiveTestDelta(t, rebuiltStream)
	if len(known.Resources) != 0 || len(known.RemovedResources) != 0 {
		t.Errorf("Delta ADS(rebuilt cache, known versions) = %v, want empty initial difference", known)
	}

	publishTestAIResources(t, publisher, nil)
	removed := receiveTestDelta(t, resumed)
	slices.Sort(removed.RemovedResources)
	if len(removed.Resources) != 0 || !slices.Equal(removed.RemovedResources, []string{
		"entry/8080/gateway/team-a/public/listener/api", "model/team-a/b/rule/0",
	}) {
		t.Errorf("Delta ADS(AI Proxy empty configuration) = %v, want remaining entry and rule removed", removed)
	}

	currentEnvoy := publishTestSnapshot(t, publisher, twoPortConfig())
	if currentEnvoy != initialEnvoy {
		t.Error("AI Proxy resource updates replaced unchanged Envoy snapshot")
	}
}

func TestADSMuxSeparatesTypesAndRejectsOtherGroups(t *testing.T) {
	publisher := NewPublisher("", "")
	publishTestAIResources(t, publisher, aiTestResources())
	publishTestSnapshot(t, publisher, twoPortConfig())
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(Envoy through Mux) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "envoy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: resourcev3.RouteType, ResourceNamesSubscribe: []string{"*"},
	})
	response := receiveTestDelta(t, stream)
	if response.TypeUrl != resourcev3.RouteType || len(response.Resources) != 2 {
		t.Errorf("Delta ADS(Envoy through Mux) = %v, want two RDS resources and no AI Proxy resources", response)
	}

	for _, tt := range []struct {
		typeURL string
		cluster string
	}{
		{typeURL: aidataplanev1.ResourceType, cluster: "other-group"},
		{typeURL: resourcev3.RouteType, cluster: "other-group"},
		{typeURL: "unsupported", cluster: sharedDataPlaneCluster},
	} {
		rejected, err := client.DeltaAggregatedResources(ctx)
		if err != nil {
			t.Fatalf("DeltaAggregatedResources(other group) error = %v", err)
		}

		sendTestDelta(t, rejected, &discoveryv3.DeltaDiscoveryRequest{
			Node:    &corev3.Node{Id: "other", Cluster: tt.cluster},
			TypeUrl: tt.typeURL, ResourceNamesSubscribe: []string{"*"},
		})
		got, err := rejected.Recv()
		if err == nil || status.Code(err) == codes.DeadlineExceeded {
			t.Errorf("Delta ADS(%s, %s) = %v, %v, want immediate rejection without resources", tt.cluster, tt.typeURL, got, err)
		}
	}
}

func TestAIProxyADSRejectsUninitializedAndAcceptsValidEmpty(t *testing.T) {
	publisher := NewPublisher("", "")
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(uninitialized AI Proxy) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"},
	})
	if response, err := stream.Recv(); err == nil || status.Code(err) == codes.DeadlineExceeded {
		t.Fatalf("Delta ADS(uninitialized AI Proxy) = %v, %v, want rejection rather than empty configuration", response, err)
	}

	publishTestAIResources(t, publisher, nil)
	ready, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(valid empty AI Proxy) error = %v", err)
	}

	sendTestDelta(t, ready, &discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: sharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"},
	})
	response := receiveTestDelta(t, ready)
	if len(response.Resources) != 0 || len(response.RemovedResources) != 0 {
		t.Errorf("Delta ADS(valid empty AI Proxy) = %v, want empty initial response", response)
	}
}

func TestAIResourceCacheOwnsAndReusesResources(t *testing.T) {
	resources := aiTestResources()
	cache := newAIResourceCache()
	if err := cache.publish(resources); err != nil {
		t.Fatalf("publish(initial AI Proxy resources) error = %v", err)
	}

	before := cache.cache.GetResources()
	want := proto.Clone(before[resources[1].Name])
	resources[1].GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{Exact: "caller mutation"}
	if diff := cmp.Diff(want, before[resources[1].Name], protocmp.Transform()); diff != "" {
		t.Errorf("publish() retained caller storage (-want +got):\n%s", diff)
	}

	resources = aiTestResources()
	slices.Reverse(resources)
	if err := cache.publish(resources); err != nil {
		t.Fatalf("publish(equal resources) error = %v", err)
	}

	after := cache.cache.GetResources()
	for name, resource := range before {
		if after[name] != resource {
			t.Errorf("publish(equal resources) replaced %q", name)
		}
	}

	// 编码失败或重名必须在安装前拒绝，不删除原缓存中的其他资源。
	for _, invalid := range [][]*aidataplanev1.Resource{
		{resources[0], resources[0]},
		{{Name: "invalid", Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{HostName: "\xff"}}}},
	} {
		if err := cache.publish(invalid); err == nil {
			t.Error("publish(invalid candidate) error = nil, want failure")
		}

		if diff := cmp.Diff(before, cache.cache.GetResources(), protocmp.Transform()); diff != "" {
			t.Errorf("publish(invalid candidate) changed cache (-want +got):\n%s", diff)
		}
	}
}

func aiTestResources() []*aidataplanev1.Resource {
	const hostName = "gateway/team-a/public/listener/api"
	resources := []*aidataplanev1.Resource{{
		Name:    "entry/8080/" + hostName,
		Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{Port: 8080, HostName: hostName}},
	}}
	for _, name := range []string{"a", "b"} {
		resources = append(resources, &aidataplanev1.Resource{
			Name: "model/team-a/" + name + "/rule/0",
			Payload: &aidataplanev1.Resource_ModelRule{ModelRule: &aidataplanev1.ModelRule{
				EntryRefs: []*aidataplanev1.EntryReference{{Port: 8080, HostName: hostName}},
				Origin:    &aidataplanev1.Origin{Namespace: "team-a", Name: name, CreationTime: timestamppb.New(time.Unix(1, 0))},
				Matches:   []*aidataplanev1.ModelMatch{{Condition: &aidataplanev1.ModelMatch_Exact{Exact: name}}},
				Outcome:   &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}},
			}},
		})
	}

	return resources
}

func publishTestAIResources(t *testing.T, publisher *Publisher, resources []*aidataplanev1.Resource) {
	t.Helper()
	if err := publisher.aiResources.publish(resources); err != nil {
		t.Fatalf("publish(AI Proxy resources) error = %v", err)
	}
}
