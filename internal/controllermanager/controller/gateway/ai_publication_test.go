package gateway

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane/xds"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestGatewayDeletionTransfersPublicAIEntries(t *testing.T) {
	snapshot := aiResourceSnapshot()
	owner := *snapshot.gateways.Items[0].DeepCopy()
	owner.Name = "earlier"
	owner.CreationTimestamp = metav1.Unix(0, 0)
	owner.Finalizers = []string{"example.com/preserve"}
	apiClient := newFakeClient(t, &owner, &snapshot.gateways.Items[0])
	for _, gatewayName := range []string{"earlier", "public"} {
		route := routeResource(gatewayName, 1, gatewayName, "api", "model-network", gatewayv1alpha1.PathMatchPrefix)
		route.Namespace = "team-a"
		snapshot.httpRoutes.Items = append(snapshot.httpRoutes.Items, route)
	}

	publisher := xds.NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	connection := newPublicationADSClient(t, publisher)
	for _, phase := range []struct {
		name       string
		deleting   bool
		gateway    string
		entryPaths []string
	}{
		{name: "live owner rejects conflicting Gateway", gateway: "earlier"},
		{
			name:       "deleting owner releases public model and MCP entries",
			deleting:   true,
			gateway:    "public",
			entryPaths: []string{aidataplane.ModelChatCompletionsPath, aidataplane.ModelResponsesPath, "/mcp"},
		},
	} {
		t.Run(phase.name, func(t *testing.T) {
			if phase.deleting {
				if err := apiClient.Delete(t.Context(), &owner); err != nil {
					t.Fatalf("Delete(owner with finalizer) error = %v", err)
				}
			}

			if err := apiClient.List(t.Context(), &snapshot.gateways); err != nil {
				t.Fatalf("List(Gateways) error = %v", err)
			}

			if len(snapshot.gateways.Items) != 2 {
				t.Fatalf("List(Gateways) count = %d, want both retained objects", len(snapshot.gateways.Items))
			}

			for _, gateway := range snapshot.gateways.Items {
				if gateway.Name == owner.Name && gateway.DeletionTimestamp.IsZero() == phase.deleting {
					t.Fatalf("owner deletion timestamp = %v, want deleting %t", gateway.DeletionTimestamp, phase.deleting)
				}
			}

			compiled := snapshot.compiled()
			resolved := resolve(compiled.gateways.Items, compiled.httpRoutes, compiled.backends)
			if !phase.deleting {
				if len(resolved.gatewayConflicts) != 1 || resolved.gatewayConflicts[0].gateway.Name != "public" {
					t.Errorf("resolve(live owner) conflicts = %+v, want public rejected", resolved.gatewayConflicts)
				}
			}

			ai, err := compileAI(compiled)
			if err != nil {
				t.Fatalf("compileAI() error = %v", err)
			}

			if err := publisher.Publish(t.Context(), compile(resolved)); err != nil {
				t.Fatalf("Publish(HTTP compilation) error = %v", err)
			}

			reconciler := New(apiClient, apiClient, publisher, "control-plane")
			setTestResources(t, reconciler, snapshot)
			resources, err := reconciler.sources.readPublication().aiResources()
			if err != nil {
				t.Fatalf("aiResources() error = %v", err)
			}

			if err := publisher.PublishAI(t.Context(), ai, resources); err != nil {
				t.Fatalf("PublishAI(AI compilation) error = %v", err)
			}

			routes := receivePublishedPublicRoutes(t, connection)
			if len(routes.VirtualHosts) != 1 {
				t.Fatalf("published public hosts = %d, want one", len(routes.VirtualHosts))
			}

			host := routes.VirtualHosts[0]
			wantHost := "gateway/team-a/" + phase.gateway + "/listener/api"
			if host.Name != wantHost || !slices.Equal(host.Domains, []string{"gateway.example.com"}) {
				t.Errorf("published public host = %s/%v, want %s/[gateway.example.com]", host.Name, host.Domains, wantHost)
			}

			if len(host.Routes) != len(phase.entryPaths)+1 {
				t.Fatalf("published public routes = %v, want AI paths %v followed by ordinary HTTP", host.Routes, phase.entryPaths)
			}

			for i, entryPath := range phase.entryPaths {
				entry := host.Routes[i]
				if entry.GetMatch().GetPath() != entryPath || entry.GetRoute().GetCluster() != "ai/ingress" {
					t.Errorf("published entry %d = %v, want exact %s to ai/ingress", i, entry, entryPath)
				}
			}

			ordinary := host.Routes[len(phase.entryPaths)]
			wantRoute := "team-a/" + phase.gateway + "/rule/0/match/0"
			if ordinary.Name != wantRoute || ordinary.GetMatch().GetPathSeparatedPrefix() != "/api" || ordinary.GetRoute().GetCluster() != "backend/team-a/model-network" {
				t.Errorf("published ordinary route = %v, want %s /api to same-namespace backend", ordinary, wantRoute)
			}
		})
	}
}

func TestReconcilePublishesAIResourcesOverADS(t *testing.T) {
	fixtures := aiResourceSnapshot()
	apiClient := aiResourceClient(t, fixtures)
	publisher := xds.NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	reconciler := newTestReconciler(t, apiClient, publisher)
	connection := newPublicationADSClient(t, publisher)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if _, err := reconciler.Reconcile(ctx, sharedRequest()); err != nil {
		t.Fatalf("Reconcile(AI Proxy publication) error = %v", err)
	}

	stream, err := discoveryv3.NewAggregatedDiscoveryServiceClient(connection).DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(AI Proxy publication) error = %v", err)
	}

	if err := stream.Send(&discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "aiproxy-1", Cluster: dataplane.SharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"},
	}); err != nil {
		t.Fatalf("Send(AI Proxy publication) error = %v", err)
	}

	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv(AI Proxy publication) error = %v", err)
	}

	if response.TypeUrl != aidataplanev1.ResourceType || len(response.Resources) != 3 || len(response.RemovedResources) != 0 {
		t.Fatalf("Recv(AI Proxy publication) = %v, want entry, model rule and MCP route", response)
	}

	got := make(map[string]*aidataplanev1.Resource)
	for _, resource := range response.Resources {
		var decoded aidataplanev1.Resource
		if err := resource.Resource.UnmarshalTo(&decoded); err != nil {
			t.Fatalf("UnmarshalTo(AI Proxy %q) error = %v", resource.Name, err)
		}

		got[resource.Name] = &decoded
	}

	want := compiledAIResources(t, reconciler.sources.readPublication())
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Reconcile → AI Proxy ADS mismatch (-want +got):\n%s", diff)
	}
}

// newPublicationADSClient 从公开 ADS 边界读取真实 Publisher，不暴露生产缓存给测试。
func newPublicationADSClient(t *testing.T, publisher *xds.Publisher) *grpc.ClientConn {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(temporary ADS address) error = %v", err)
	}

	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close(temporary ADS address) error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	server := xds.NewServer(address, publisher, logr.Discard())
	finished := make(chan error, 1)
	go func() { finished <- server.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("Start(ADS) error = %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Start(ADS) did not stop within 3s")
		}
	})

	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient(ADS) error = %v", err)
	}

	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("Close(ADS client) error = %v", err)
		}
	})

	return connection
}

func receivePublishedPublicRoutes(t *testing.T, connection *grpc.ClientConn) *routev3.RouteConfiguration {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stream, err := discoveryv3.NewAggregatedDiscoveryServiceClient(connection).DeltaAggregatedResources(ctx, grpc.WaitForReady(true))
	if err != nil {
		t.Fatalf("DeltaAggregatedResources() error = %v", err)
	}

	if err := stream.Send(&discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: "publication-regression", Cluster: "ingate-shared-data-plane"},
		TypeUrl: resourcev3.RouteType,
	}); err != nil {
		t.Fatalf("ADS Send(public routes) error = %v", err)
	}

	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("ADS Recv(public routes) error = %v", err)
	}

	for _, resource := range response.Resources {
		var routes routev3.RouteConfiguration
		if err := resource.Resource.UnmarshalTo(&routes); err != nil {
			t.Fatalf("UnmarshalTo(published routes) error = %v", err)
		}

		if routes.Name == "route/8080" {
			return &routes
		}
	}

	t.Fatal("ADS response is missing public route/8080")

	return nil
}
