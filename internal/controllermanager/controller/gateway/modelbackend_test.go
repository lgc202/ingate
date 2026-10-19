package gateway

import (
	"sync/atomic"
	"testing"
	"testing/synctest"

	"istio.io/istio/pkg/kube/krt"
)

func TestModelBackendBindingTracksExactDependency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		protocol := aiResourceSnapshot().aiBackends.Items[0]
		protocols := krt.NewMutableCollection(nil, []*modelBackend{compileModelBackend(&protocol)}, krt.WithStop(t.Context().Done()))
		networks := krt.NewMutableCollection[*networkBackend](nil, nil, krt.WithStop(t.Context().Done()))
		var calls atomic.Int64
		bind := func(ctx krt.HandlerContext, backend *modelBackend) *modelBackendBinding {
			calls.Add(1)

			return bindModelBackend(ctx, backend, networks.AsCollection())
		}
		bindings := krt.NewPointerCollection(protocols.AsCollection(), bind, krt.WithStop(t.Context().Done()))
		if !bindings.WaitUntilSynced(t.Context().Done()) {
			t.Fatal("binding collection did not sync")
		}

		initial := *bindings.GetKey("team-a/model-service")
		if initial.network != nil {
			t.Errorf("missing target binding = %+v, want unavailable", initial)
		}

		backend := backendResource("model-network")
		backend.Namespace = "team-b"
		networks.UpdateObject(compileNetworkBackend(&backend))
		synctest.Wait()

		if got := *bindings.GetKey("team-a/model-service"); got != initial {
			t.Error("another Namespace changed the binding, want unchanged unavailable result")
		}

		if got := calls.Load(); got != 1 {
			t.Errorf("binding conversions after other Namespace update = %d, want unchanged 1", got)
		}

		backend.Namespace = "team-a"
		networks.UpdateObject(compileNetworkBackend(&backend))
		synctest.Wait()

		restored := *bindings.GetKey("team-a/model-service")
		if restored.network == nil || restored.network.key.Namespace != "team-a" {
			t.Fatalf("target created binding = %+v, want team-a/model-network", restored)
		}

		// 改绑到尚不存在的目标，旧目标的更新不得继续驱动这个绑定。
		protocol.Spec.BackendRef.Name = "replacement"
		protocols.UpdateObject(compileModelBackend(&protocol))
		synctest.Wait()
		retargeted := *bindings.GetKey("team-a/model-service")
		if retargeted.network != nil {
			t.Fatal("missing replacement target remained available")
		}

		retargetCalls := calls.Load()
		backend.Spec.Endpoints[0].Weight++
		networks.UpdateObject(compileNetworkBackend(&backend))
		synctest.Wait()

		if got := *bindings.GetKey("team-a/model-service"); got != retargeted {
			t.Error("old dependency update changed the retargeted binding, want same result instance")
		}

		if got := calls.Load(); got != retargetCalls {
			t.Errorf("old dependency reran binding conversion = %d, want unchanged %d", got, retargetCalls)
		}

		backend.Name = "replacement"
		networks.UpdateObject(compileNetworkBackend(&backend))
		synctest.Wait()

		if got := *bindings.GetKey("team-a/model-service"); got.network == nil || got.network.key.Name != "replacement" {
			t.Fatalf("replacement created binding = %+v, want replacement network", got)
		}

		networks.DeleteObject("team-a/replacement")
		synctest.Wait()
		if got := *bindings.GetKey("team-a/model-service"); got.network != nil {
			t.Errorf("replacement deleted binding = %+v, want unavailable", got)
		}

		networks.UpdateObject(compileNetworkBackend(&backend))
		synctest.Wait()
		if got := *bindings.GetKey("team-a/model-service"); got.network == nil {
			t.Error("replacement recreated binding is unavailable, want recovered target")
		}
	})
}
