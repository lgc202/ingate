package envoy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestHTTPTranslationReusesUnchangedResults(t *testing.T) {
	config := twoPortConfig()
	config.Backends[0].TLS = &dataplane.BackendTLSSettings{SNI: "orders.example.com"}
	var empty httpTranslation

	first := compileTestHTTP(t, &empty, config)
	again := compileTestHTTP(t, &first, config)

	// 在 Snapshot 去重之前比较转换结果指针；构造函数会创建新消息，不能以哈希相等代替未重译。
	if first.listeners[8080] != again.listeners[8080] || first.routes[8080].resource != again.routes[8080].resource ||
		first.backends["orders"].cluster != again.backends["orders"].cluster {
		t.Error("compile(unchanged configuration) rebuilt cached protobuf messages")
	}

	config.Listeners[0].Hosts[0].Routes[0].Path = "/v2/orders"

	changed := compileTestHTTP(t, &again, config)

	if changed.routes[8080].resource == again.routes[8080].resource {
		t.Error("compile(changed route) reused stale RDS")
	}

	if first.routes[8080].source.Hosts[0].Routes[0].Path != "/orders" {
		t.Error("caller mutation changed the cached route declaration")
	}

	if changed.listeners[8080] != again.listeners[8080] || changed.routes[9090].resource != again.routes[9090].resource ||
		changed.backends["orders"].cluster != again.backends["orders"].cluster ||
		changed.backends["operations"].endpoints != again.backends["operations"].endpoints {
		t.Error("compile(changed route) rebuilt unrelated Listener, RDS or Backend")
	}

	config.Backends[0].Endpoints[0].Address = "192.0.2.3"

	changedNetwork := compileTestHTTP(t, &changed, config)

	if changedNetwork.backends["orders"].endpoints == changed.backends["orders"].endpoints {
		t.Error("compile(changed endpoint) reused stale assignment")
	}

	if changedNetwork.routes[8080].resource != changed.routes[8080].resource ||
		changedNetwork.routes[9090].resource != changed.routes[9090].resource ||
		changedNetwork.backends["operations"].cluster != changed.backends["operations"].cluster {
		t.Error("compile(changed endpoint) rebuilt unrelated routes or Backend")
	}

	config.Backends[0].TLS.SNI = "changed.example.com"

	changedTLS := compileTestHTTP(t, &changedNetwork, config)

	if changedTLS.backends["orders"].cluster == changedNetwork.backends["orders"].cluster {
		t.Error("compile(changed SNI) reused stale Cluster")
	}

	if first.backends["orders"].source.TLS.SNI != "orders.example.com" ||
		first.backends["orders"].source.Endpoints[0].Address != "192.0.2.1" {
		t.Error("caller mutation changed cached TLS or endpoint declaration")
	}
}

func TestHTTPTranslationWithdrawsDeletedResources(t *testing.T) {
	config := twoPortConfig()
	var empty httpTranslation

	first := compileTestHTTP(t, &empty, config)

	remaining := dataplane.Config{Listeners: config.Listeners[1:], Backends: config.Backends[1:]}

	deleted := compileTestHTTP(t, &first, remaining)

	if len(deleted.listeners) != 1 || len(deleted.routes) != 1 || len(deleted.backends) != 1 ||
		deleted.listeners[8080] != nil || deleted.backends["orders"].cluster != nil {
		t.Error("compile(deletion) retained deleted cache entries")
	}

	if deleted.routes[9090].resource != first.routes[9090].resource ||
		deleted.backends["operations"].cluster != first.backends["operations"].cluster {
		t.Error("compile(deletion) rebuilt surviving entries")
	}

	recreated := compileTestHTTP(t, &deleted, config)

	if recreated.listeners[8080] == first.listeners[8080] || recreated.backends["orders"].cluster == first.backends["orders"].cluster {
		t.Error("compile(recreation) recovered an obsolete historical cache entry")
	}

	config.Backends[0].Endpoints[0].Address = "orders.example.com"

	dns := compileTestHTTP(t, &recreated, config)

	if dns.backends["orders"].cluster.GetType() != clusterv3.Cluster_STRICT_DNS || len(dns.resources.endpoints) != 1 {
		t.Error("compile(DNS transition) did not replace EDS with the embedded DNS assignment")
	}

	config.Backends[0].Endpoints[0].Address = "192.0.2.1"

	ip := compileTestHTTP(t, &dns, config)

	if ip.backends["orders"].cluster.GetType() != clusterv3.Cluster_EDS || len(ip.resources.endpoints) != 2 {
		t.Error("compile(IP transition) did not restore EDS")
	}

	cleared := compileTestHTTP(t, &ip, dataplane.Config{})

	if len(cleared.listeners) != 0 || len(cleared.routes) != 0 || len(cleared.backends) != 0 {
		t.Error("compile(empty) retained translation cache entries")
	}
}

func TestPublishRetainsTranslationAfterRejectedCandidate(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	first := publishTestSnapshot(t, publisher, config)
	previousRoute := publisher.http.routes[8080].resource
	previousBackend := publisher.http.backends["operations"].cluster
	previousIngress := publisher.ingress.byName["route/8080"].resource

	// 先成功生成新 RDS，再让 Backend 转换失败；不能把部分候选写进缓存。
	config.Listeners[0].Hosts[0].Routes[0].Path = "/changed"
	config.Backends[1].Endpoints[0].Port = 70000

	if err := publisher.Publish(t.Context(), config); err == nil {
		t.Fatal("Publish(invalid endpoint) error = nil, want candidate rejected")
	}

	if publisher.http.routes[8080].resource != previousRoute || publisher.http.backends["operations"].cluster != previousBackend {
		t.Error("failed conversion partially replaced HTTP translation cache")
	}

	// protobuf 本身有效但缺失 Cluster，必须在跨类型引用校验失败后同样保留缓存。
	config.Backends[1].Endpoints[0].Port = 9000
	config.Listeners[0].Hosts[0].Routes[0].BackendName = "missing"

	if err := publisher.Publish(t.Context(), config); err == nil {
		t.Fatal("Publish(missing Cluster reference) error = nil, want candidate rejected")
	}

	if publisher.http.routes[8080].resource != previousRoute || publisher.ingress.byName["route/8080"].resource != previousIngress {
		t.Error("failed snapshot validation replaced HTTP or ingress translation cache")
	}

	emptyAI := aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	}

	if err := publisher.PublishAI(t.Context(), emptyAI); err != nil {
		t.Fatalf("PublishAI(after rejected HTTP) error = %v, want original HTTP state usable", err)
	}

	current, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after rejected candidates) error = %v", err)
	}

	if current != first {
		t.Error("rejected candidates changed the previous valid snapshot")
	}
}

func TestPublishRetainsTranslationAfterNotificationFailure(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	initial := publishTestSnapshot(t, publisher, config)
	responses := make(chan cachev3.Response)
	subscription := streamv3.NewSotwSubscription(nil, true)
	// 同版本仍可能触发首次订阅响应，必须登记客户端已经持有的资源。
	subscription.SetReturnedResources(initial.GetVersionMap(resourcev3.EndpointType))
	cancelWatch, err := publisher.cache.CreateWatch(&cachev3.Request{
		Node:        &corev3.Node{Cluster: sharedDataPlaneCluster},
		TypeUrl:     resourcev3.EndpointType,
		VersionInfo: initial.GetVersion(resourcev3.EndpointType),
	}, subscription, responses)
	if err != nil || cancelWatch == nil {
		t.Fatalf("CreateWatch(EDS current version) error/cancel = %v/%t, want pending watch", err, cancelWatch != nil)
	}

	t.Cleanup(cancelWatch)
	config.Backends[0].Endpoints[0].Address = "192.0.2.3"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = publisher.Publish(ctx, config)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish(cancelled notification) error = %v, want context.Canceled", err)
	}

	installed := publisher.http.backends["orders"]
	if installed.source.Endpoints[0].Address != "192.0.2.3" {
		t.Fatal("notification failure rolled back the new translation cache")
	}

	// 原生 Cache 已安装新快照，重试相同内容也必须再次通知，且不重译 Backend。
	retryCtx, retryCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer retryCancel()

	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Go(func() { result <- publisher.Publish(retryCtx, config) })
	t.Cleanup(worker.Wait)

	select {
	case response := <-responses:
		if len(response.GetReturnedResources()) != 2 {
			t.Errorf("retry EDS Watch resources = %d, want 2", len(response.GetReturnedResources()))
		}
	case <-retryCtx.Done():
		t.Fatal("Publish(unchanged retry) did not resume pending EDS Watch")
	}

	if err := <-result; err != nil {
		t.Fatalf("Publish(retry notification) error = %v, want nil", err)
	}

	if publisher.http.backends["orders"].cluster != installed.cluster || publisher.http.backends["orders"].endpoints != installed.endpoints {
		t.Error("Publish(notification retry) rebuilt unchanged Backend")
	}
}

func compileTestHTTP(t *testing.T, previous *httpTranslation, config dataplane.Config) httpTranslation {
	t.Helper()
	translated, err := previous.compile(config)
	if err != nil {
		t.Fatalf("compile(HTTP configuration) error = %v", err)
	}

	want, err := buildResources(config)
	if err != nil {
		t.Fatalf("buildResources(reference) error = %v", err)
	}

	if diff := cmp.Diff(want, translated.resources, cmp.AllowUnexported(resourceSet{}), protocmp.Transform()); diff != "" {
		t.Fatalf("incremental HTTP translation mismatch (-want +got):\n%s", diff)
	}

	return translated
}
