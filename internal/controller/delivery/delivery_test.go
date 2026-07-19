package delivery

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"k8s.io/apimachinery/pkg/types"

	"github.com/lgc202/ingate/internal/controller/compiler"
	"github.com/lgc202/ingate/internal/controller/xds"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

type setSnapshotErrorCache struct {
	cachev3.SnapshotCache
}

type failNextSetSnapshotCache struct {
	cachev3.SnapshotCache

	failNext bool
	setCalls int
}

func (c *setSnapshotErrorCache) SetSnapshot(
	ctx context.Context,
	node string,
	snapshot cachev3.ResourceSnapshot,
) error {
	if err := c.SnapshotCache.SetSnapshot(ctx, node, snapshot); err != nil {
		return err
	}
	return errors.New("reported snapshot publication failure")
}

func (c *failNextSetSnapshotCache) SetSnapshot(
	ctx context.Context,
	node string,
	snapshot cachev3.ResourceSnapshot,
) error {
	c.setCalls++
	if c.failNext {
		c.failNext = false
		return errors.New("set snapshot failed")
	}
	return c.SnapshotCache.SetSnapshot(ctx, node, snapshot)
}

func TestDeliveryAllowsNACKedVersionToBeSubmittedAgain(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}

	d.state.streams[1] = &streamState{
		nodeID:   "envoy-1",
		versions: make(map[string]*ackProgress),
	}
	d.state.streams[1].progress(result.Version).sent[resourcev3.ListenerType] = true
	if err := d.handleNACK(context.Background(), xds.Event{
		Kind:     xds.EventNACK,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  result.Version,
	}); err != nil {
		t.Fatalf("Delivery.handleNACK(%q) error = %v", result.Version, err)
	}
	if d.state.candidate != nil {
		t.Fatalf("Delivery.handleNACK(%q) candidate = %q, want nil", result.Version, d.state.candidate.version)
	}

	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) after NACK error = %v", result.Version, err)
	}
	if d.state.candidate == nil || d.state.candidate.version != result.Version {
		t.Fatalf("Delivery.handleSubmit(%q) after NACK candidate = %#v, want same version", result.Version, d.state.candidate)
	}
}

func TestDeliveryNACKKeepsActivePolicyTargetsAndRecordsCandidateTargets(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	activeTarget := compiler.CompiledPolicyTarget{
		Policy: compiler.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: "rate-limit-1", Generation: 1},
		Target: compiler.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway-1", Generation: 1},
	}
	active := compiler.Result{Version: "version-active", PolicyTargets: []compiler.CompiledPolicyTarget{activeTarget}}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
	}
	d.activateCandidate()

	candidateTarget := activeTarget
	candidateTarget.Target.Generation = 2
	candidate := compiler.Result{Version: "version-candidate", PolicyTargets: []compiler.CompiledPolicyTarget{candidateTarget}}
	if err := d.handleSubmit(context.Background(), candidate, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", candidate.Version, err)
	}
	d.state.streams[1] = &streamState{nodeID: "envoy-1", versions: make(map[string]*ackProgress)}
	d.state.streams[1].progress(candidate.Version).sent[resourcev3.ListenerType] = true
	if err := d.handleNACK(context.Background(), xds.Event{
		Kind:     xds.EventNACK,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  candidate.Version,
	}); err != nil {
		t.Fatalf("Delivery.handleNACK(%q) error = %v", candidate.Version, err)
	}

	if got, want := d.state.active.policyTargets, active.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleNACK(%q) active policy targets = %v, want %v", candidate.Version, got, want)
	}
	if got, want := d.state.lastFailure.PolicyTargets, candidate.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleNACK(%q) failure policy targets = %v, want %v", candidate.Version, got, want)
	}
}

func TestAffectedPolicyTargetsIncludesRemovedActiveTargetAtDesiredGeneration(t *testing.T) {
	active := &publishedConfig{policyTargets: []compiler.CompiledPolicyTarget{{
		Policy: compiler.ResourceGeneration{
			Kind:       gatewayv1.KindRateLimitPolicy,
			Name:       "rate-limit-1",
			UID:        types.UID("policy-uid"),
			Generation: 1,
		},
		Target: compiler.ResourceGeneration{
			Kind:       gatewayv1.KindGateway,
			Name:       "gateway-1",
			UID:        types.UID("gateway-uid"),
			Generation: 1,
		},
	}}}
	desiredPolicy := active.policyTargets[0].Policy
	desiredPolicy.Generation = 2
	desiredTarget := active.policyTargets[0].Target
	desiredTarget.Generation = 2

	got := affectedPolicyTargets(active, []compiler.ResourceGeneration{desiredPolicy, desiredTarget}, nil)
	want := []compiler.CompiledPolicyTarget{{Policy: desiredPolicy, Target: desiredTarget}}
	if !slices.Equal(got, want) {
		t.Errorf("affectedPolicyTargets(removed active target) = %v, want %v", got, want)
	}
}

func TestDeliveryNACKRecordsRemovedActivePolicyTargetAtDesiredGeneration(t *testing.T) {
	tests := []struct {
		name             string
		desiredPolicyGen int64
		desiredTargetGen int64
	}{
		{
			name:             "policy disabled",
			desiredPolicyGen: 2,
			desiredTargetGen: 1,
		},
		{
			name:             "target disabled",
			desiredPolicyGen: 1,
			desiredTargetGen: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			activeTarget := compiler.CompiledPolicyTarget{
				Policy: compiler.ResourceGeneration{
					Kind:       gatewayv1.KindRateLimitPolicy,
					Name:       "rate-limit-1",
					UID:        types.UID("policy-uid"),
					Generation: 1,
				},
				Target: compiler.ResourceGeneration{
					Kind:       gatewayv1.KindGateway,
					Name:       "gateway-1",
					UID:        types.UID("gateway-uid"),
					Generation: 1,
				},
			}
			active := compiler.Result{
				Version:             "version-active",
				ResourceGenerations: []compiler.ResourceGeneration{activeTarget.Policy, activeTarget.Target},
				PolicyTargets:       []compiler.CompiledPolicyTarget{activeTarget},
			}
			if err := d.handleSubmit(context.Background(), active, false); err != nil {
				t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
			}
			d.activateCandidate()

			desiredPolicy := activeTarget.Policy
			desiredPolicy.Generation = tt.desiredPolicyGen
			desiredTarget := activeTarget.Target
			desiredTarget.Generation = tt.desiredTargetGen
			candidate := compiler.Result{
				Version:             "version-candidate",
				ResourceGenerations: []compiler.ResourceGeneration{desiredPolicy, desiredTarget},
			}
			if err := d.handleSubmit(context.Background(), candidate, false); err != nil {
				t.Fatalf("Delivery.handleSubmit(%q) error = %v", candidate.Version, err)
			}
			d.state.streams[1] = &streamState{nodeID: "envoy-1", versions: make(map[string]*ackProgress)}
			d.state.streams[1].progress(candidate.Version).sent[resourcev3.ListenerType] = true
			if err := d.handleNACK(context.Background(), xds.Event{
				Kind:     xds.EventNACK,
				StreamID: 1,
				NodeID:   "envoy-1",
				TypeURL:  resourcev3.ListenerType,
				Version:  candidate.Version,
			}); err != nil {
				t.Fatalf("Delivery.handleNACK(%q) error = %v", candidate.Version, err)
			}

			if got, want := d.state.active.policyTargets, active.PolicyTargets; !slices.Equal(got, want) {
				t.Errorf("Delivery.handleNACK(%q) active policy targets = %v, want %v", candidate.Version, got, want)
			}
			wantFailureTargets := []compiler.CompiledPolicyTarget{{Policy: desiredPolicy, Target: desiredTarget}}
			if got := d.state.lastFailure.PolicyTargets; !slices.Equal(got, wantFailureTargets) {
				t.Errorf("Delivery.handleNACK(%q) failure policy targets = %v, want %v", candidate.Version, got, wantFailureTargets)
			}
		})
	}
}

func TestDeliveryACKTimeoutRecordsRemovedActivePolicyTarget(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	activeTarget := compiler.CompiledPolicyTarget{
		Policy: compiler.ResourceGeneration{
			Kind:       gatewayv1.KindAccessControlPolicy,
			Name:       "access-control-1",
			UID:        types.UID("policy-uid"),
			Generation: 1,
		},
		Target: compiler.ResourceGeneration{
			Kind:       gatewayv1.KindRoute,
			Name:       "route-1",
			UID:        types.UID("route-uid"),
			Generation: 1,
		},
	}
	active := compiler.Result{
		Version:             "version-active",
		ResourceGenerations: []compiler.ResourceGeneration{activeTarget.Policy, activeTarget.Target},
		PolicyTargets:       []compiler.CompiledPolicyTarget{activeTarget},
	}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
	}
	d.activateCandidate()

	desiredPolicy := activeTarget.Policy
	desiredPolicy.Generation = 2
	candidate := compiler.Result{
		Version:             "version-candidate",
		ResourceGenerations: []compiler.ResourceGeneration{desiredPolicy, activeTarget.Target},
	}
	if err := d.handleSubmit(context.Background(), candidate, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", candidate.Version, err)
	}
	d.handleACKTimeout(candidate.Version, d.state.candidate.sequence)

	wantFailureTargets := []compiler.CompiledPolicyTarget{{Policy: desiredPolicy, Target: activeTarget.Target}}
	if got := d.state.lastFailure.PolicyTargets; !slices.Equal(got, wantFailureTargets) {
		t.Errorf("Delivery.handleACKTimeout(%q) failure policy targets = %v, want %v", candidate.Version, got, wantFailureTargets)
	}
	if got, want := d.state.active.policyTargets, active.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleACKTimeout(%q) active policy targets = %v, want %v", candidate.Version, got, want)
	}
}

func TestDeliveryCancelCandidateRestoresBaseline(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	if err := d.handleCancelCandidate(context.Background()); err != nil {
		t.Fatalf("Delivery.handleCancelCandidate(%q) error = %v", result.Version, err)
	}

	if d.state.candidate != nil {
		t.Fatalf("Delivery.handleCancelCandidate(%q) candidate = %#v, want nil", result.Version, d.state.candidate)
	}
	if !d.cacheHasVersion(BaselineVersion) {
		t.Errorf("Delivery.handleCancelCandidate(%q) cache version is not baseline", result.Version)
	}
}

func TestDeliveryLateACKActivatesTimedOutCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resource := compiler.ResourceGeneration{
		Kind:       "Upstream",
		Name:       "upstream-1",
		UID:        types.UID("upstream-uid"),
		Generation: 1,
	}
	result := compiler.Result{
		Version:             "version-1",
		ResourceGenerations: []compiler.ResourceGeneration{resource},
	}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.state.streams[1] = &streamState{
		nodeID:   "envoy-1",
		versions: make(map[string]*ackProgress),
	}
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  result.Version,
	})
	d.handleACKTimeout(result.Version, d.state.candidate.sequence)
	if d.state.lastFailure == nil || d.state.lastFailure.Reason != FailureDelivery {
		t.Fatalf("Delivery.handleACKTimeout(%q) last failure = %#v, want delivery failure", result.Version, d.state.lastFailure)
	}

	d.handleACK(xds.Event{
		Kind:            xds.EventACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		Version:         result.Version,
		AcceptedVersion: result.Version,
	})
	if d.state.candidate != nil {
		t.Fatalf("Delivery.handleACK(%q) candidate = %#v, want nil", result.Version, d.state.candidate)
	}
	if d.state.active == nil || d.state.active.version != result.Version {
		t.Fatalf("Delivery.handleACK(%q) active = %#v, want active version", result.Version, d.state.active)
	}
	if d.state.lastFailure != nil {
		t.Errorf("Delivery.handleACK(%q) last failure = %#v, want nil", result.Version, d.state.lastFailure)
	}
}

func TestDeliverySubmittingActiveVersionCancelsCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	active := compiler.Result{Version: "version-active"}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
	}
	d.activateCandidate()

	candidate := compiler.Result{Version: "version-candidate"}
	if err := d.handleSubmit(context.Background(), candidate, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", candidate.Version, err)
	}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) while %q is pending error = %v", active.Version, candidate.Version, err)
	}

	if d.state.candidate != nil {
		t.Fatalf("Delivery.handleSubmit(%q) candidate = %#v, want nil", active.Version, d.state.candidate)
	}
	if d.state.active == nil || d.state.active.version != active.Version {
		t.Fatalf("Delivery.handleSubmit(%q) active = %#v, want active version", active.Version, d.state.active)
	}
	if !d.cacheHasVersion(active.Version) {
		t.Errorf("Delivery.handleSubmit(%q) cache version is not active", active.Version)
	}
}

func TestDeliveryRetriesFailedActiveFallbackBeforeUpdatingProvenance(t *testing.T) {
	snapshotCache := &failNextSetSnapshotCache{
		SnapshotCache: cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil),
	}
	d, err := New(snapshotCache, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	oldActive := compiler.ResourceGeneration{
		Kind:       "Upstream",
		Name:       "upstream-1",
		UID:        types.UID("upstream-uid"),
		Generation: 1,
	}
	active := compiler.Result{
		Version:             "version-active",
		ResourceGenerations: []compiler.ResourceGeneration{oldActive},
	}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
	}
	d.activateCandidate()

	candidate := compiler.Result{Version: "version-candidate"}
	if err := d.handleSubmit(context.Background(), candidate, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", candidate.Version, err)
	}

	desiredActive := oldActive
	desiredActive.Generation = 2
	active.ResourceGenerations = []compiler.ResourceGeneration{desiredActive}
	snapshotCache.failNext = true
	if err := d.handleSubmit(context.Background(), active, false); err == nil {
		t.Fatalf("Delivery.handleSubmit(%q) fallback error = nil, want failure", active.Version)
	}
	if d.state.candidate != nil {
		t.Fatalf("Delivery.handleSubmit(%q) candidate = %#v, want nil after failed fallback", active.Version, d.state.candidate)
	}
	if got, want := d.state.active.resources, []compiler.ResourceGeneration{oldActive}; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) active resources = %v, want old provenance %v", active.Version, got, want)
	}
	if d.state.lastFailure == nil || d.state.lastFailure.Reason != FailureDelivery {
		t.Fatalf("Delivery.handleSubmit(%q) last failure = %#v, want delivery failure", active.Version, d.state.lastFailure)
	}
	if got, want := d.state.lastFailure.Resources, active.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) failure resources = %v, want desired provenance %v", active.Version, got, want)
	}
	if !d.cacheHasVersion(candidate.Version) {
		t.Errorf("Delivery.handleSubmit(%q) cache changed after failed fallback", active.Version)
	}

	setCalls := snapshotCache.setCalls
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) fallback retry error = %v", active.Version, err)
	}
	if got, want := snapshotCache.setCalls, setCalls+1; got != want {
		t.Errorf("Delivery.handleSubmit(%q) SetSnapshot calls = %d, want %d", active.Version, got, want)
	}
	if !d.cacheHasVersion(active.Version) {
		t.Errorf("Delivery.handleSubmit(%q) cache version was not restored", active.Version)
	}
	if got, want := d.state.active.resources, active.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) active resources = %v, want desired provenance %v", active.Version, got, want)
	}
	if d.state.lastFailure != nil {
		t.Errorf("Delivery.handleSubmit(%q) last failure = %#v, want nil after successful retry", active.Version, d.state.lastFailure)
	}
}

func TestDeliverySameVersionSubmitUpdatesCandidateAndActiveResources(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	first := compiler.ResourceGeneration{
		Kind:       "Upstream",
		Name:       "upstream-1",
		UID:        types.UID("upstream-uid"),
		Generation: 1,
	}
	second := first
	second.Generation = 2
	firstTarget := compiler.CompiledPolicyTarget{
		Policy: compiler.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: "rate-limit-1", Generation: 1},
		Target: compiler.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway-1", Generation: 1},
	}
	secondTarget := firstTarget
	secondTarget.Target.Generation = 2
	result := compiler.Result{
		Version:             "version-1",
		ResourceGenerations: []compiler.ResourceGeneration{first},
		PolicyTargets:       []compiler.CompiledPolicyTarget{firstTarget},
	}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.recordFailure(FailureDelivery, result.ResourceGenerations, result.PolicyTargets)

	result.ResourceGenerations = []compiler.ResourceGeneration{second}
	result.PolicyTargets = []compiler.CompiledPolicyTarget{secondTarget}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) candidate provenance update error = %v", result.Version, err)
	}
	if got, want := d.state.candidate.resources, result.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) candidate resources = %v, want %v", result.Version, got, want)
	}
	if got, want := d.state.candidate.policyTargets, result.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) candidate policy targets = %v, want %v", result.Version, got, want)
	}
	if d.state.lastFailure == nil || d.state.lastFailure.Reason != FailureDelivery {
		t.Fatalf("Delivery.handleSubmit(%q) last failure = %#v, want delivery failure preserved", result.Version, d.state.lastFailure)
	}
	if got, want := d.state.lastFailure.Resources, result.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) failure resources = %v, want %v", result.Version, got, want)
	}
	if got, want := d.state.lastFailure.PolicyTargets, result.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) failure policy targets = %v, want %v", result.Version, got, want)
	}

	d.activateCandidate()
	third := second
	third.Generation = 3
	thirdTarget := secondTarget
	thirdTarget.Target.Generation = 3
	result.ResourceGenerations = []compiler.ResourceGeneration{third}
	result.PolicyTargets = []compiler.CompiledPolicyTarget{thirdTarget}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) active provenance update error = %v", result.Version, err)
	}
	if got, want := d.state.active.resources, result.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) active resources = %v, want %v", result.Version, got, want)
	}
	if got, want := d.state.active.policyTargets, result.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.handleSubmit(%q) active policy targets = %v, want %v", result.Version, got, want)
	}
	if d.state.candidate != nil {
		t.Errorf("Delivery.handleSubmit(%q) candidate = %#v, want nil for unchanged active config", result.Version, d.state.candidate)
	}
}

func TestDeliveryAcceptedVersionObservedBeforeSubmitActivatesCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	const version = "version-1"
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: version,
	})

	result := compiler.Result{Version: version}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	assertActiveVersion(t, d, version)
}

func TestDeliveryAcceptedVersionObservedAfterSubmitActivatesCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: result.Version,
	})

	assertActiveVersion(t, d, result.Version)
}

func TestDeliveryCombinesAcceptedObservationAndACKOnSameStream(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.state.candidate.requiredTypes = []string{resourcev3.ListenerType, resourcev3.RouteType}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: result.Version,
	})
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.RouteType,
		Version:  result.Version,
	})
	d.handleACK(xds.Event{
		Kind:            xds.EventACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.RouteType,
		Version:         result.Version,
		AcceptedVersion: result.Version,
	})

	assertActiveVersion(t, d, result.Version)
}

func TestDeliveryACKWithDifferentAcceptedVersionDoesNotActivate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.state.streams[1] = &streamState{
		nodeID:           "envoy-1",
		versions:         make(map[string]*ackProgress),
		acceptedVersions: make(map[string]string),
	}
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  result.Version,
	})
	d.handleACK(xds.Event{
		Kind:            xds.EventACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		Version:         result.Version,
		AcceptedVersion: "version-old",
	})

	if d.state.active != nil {
		t.Errorf("Delivery.handleACK(%q accepted as %q) active = %#v, want nil", result.Version, "version-old", d.state.active)
	}
	if got := d.state.streams[1].acceptedVersions[resourcev3.ListenerType]; got != "" {
		t.Errorf("Delivery.handleACK(%q accepted as %q) recorded accepted version = %q, want empty", result.Version, "version-old", got)
	}
}

func TestDeliveryIncompleteAcceptedVersionObservationDoesNotActivate(t *testing.T) {
	tests := []struct {
		name            string
		typeURL         string
		acceptedVersion string
	}{
		{
			name:            "missing required type",
			typeURL:         resourcev3.RouteType,
			acceptedVersion: "version-1",
		},
		{
			name:            "different version",
			typeURL:         resourcev3.ListenerType,
			acceptedVersion: "version-old",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			result := compiler.Result{Version: "version-1"}
			if err := d.handleSubmit(context.Background(), result, false); err != nil {
				t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
			}
			if err := d.handleXDSEvent(context.Background(), xds.Event{
				Kind:     xds.EventStreamOpened,
				StreamID: 1,
				NodeID:   "envoy-1",
			}); err != nil {
				t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
			}
			d.handleAcceptedVersionObserved(xds.Event{
				Kind:            xds.EventAcceptedVersionObserved,
				StreamID:        1,
				NodeID:          "envoy-1",
				TypeURL:         tt.typeURL,
				AcceptedVersion: tt.acceptedVersion,
			})

			if d.state.active != nil {
				t.Errorf("Delivery accepted observation type %q version %q active = %#v, want nil", tt.typeURL, tt.acceptedVersion, d.state.active)
			}
			if d.state.candidate == nil {
				t.Errorf("Delivery accepted observation type %q version %q candidate = nil, want pending candidate", tt.typeURL, tt.acceptedVersion)
			}
		})
	}
}

func TestDeliveryStreamCloseClearsAcceptedVersionObservations(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	const version = "version-1"
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: version,
	})
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamClosed,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamClosed) error = %v", err)
	}

	if err := d.handleSubmit(context.Background(), compiler.Result{Version: version}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", version, err)
	}
	if d.state.active != nil {
		t.Errorf("Delivery.handleSubmit(%q) active = %#v, want nil after stream close", version, d.state.active)
	}
}

func TestDeliveryCancelClearsAcceptedVersionsBeforeFutureCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: "version-a",
	})
	if err := d.handleSubmit(context.Background(), compiler.Result{Version: "version-b"}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-b) error = %v", err)
	}
	if err := d.handleCancelCandidate(context.Background()); err != nil {
		t.Fatalf("Delivery.handleCancelCandidate() error = %v", err)
	}
	if got := len(d.state.streams[1].acceptedVersions); got != 0 {
		t.Fatalf("Delivery.handleCancelCandidate() accepted versions = %v, want empty", d.state.streams[1].acceptedVersions)
	}

	if err := d.handleSubmit(context.Background(), compiler.Result{Version: "version-a"}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-a) error = %v", err)
	}
	if d.state.active != nil {
		t.Errorf("Delivery.handleSubmit(version-a) active = %#v, want nil after clearing observations", d.state.active)
	}
}

func TestDeliveryNoopCancelPreservesAcceptedVersionObservation(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: "version-1",
	})
	if err := d.handleCancelCandidate(context.Background()); err != nil {
		t.Fatalf("Delivery.handleCancelCandidate() no-op error = %v", err)
	}
	if got := d.state.streams[1].acceptedVersions[resourcev3.ListenerType]; got != "version-1" {
		t.Fatalf("Delivery.handleCancelCandidate() no-op accepted version = %q, want version-1", got)
	}

	if err := d.handleSubmit(context.Background(), compiler.Result{Version: "version-1"}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-1) error = %v", err)
	}
	assertActiveVersion(t, d, "version-1")
}

func TestDeliveryACKReplacesOlderAcceptedVersion(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.state.streams[1] = &streamState{
		nodeID:           "envoy-1",
		versions:         make(map[string]*ackProgress),
		acceptedVersions: map[string]string{resourcev3.ListenerType: "version-a"},
	}
	if err := d.handleSubmit(context.Background(), compiler.Result{Version: "version-b"}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-b) error = %v", err)
	}
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  "version-b",
	})
	d.handleACK(xds.Event{
		Kind:            xds.EventACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		Version:         "version-b",
		AcceptedVersion: "version-b",
	})
	if got := d.state.streams[1].acceptedVersions[resourcev3.ListenerType]; got != "version-b" {
		t.Fatalf("Delivery.handleACK(version-b) accepted version = %q, want version-b", got)
	}

	if err := d.handleSubmit(context.Background(), compiler.Result{Version: "version-a"}, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-a) error = %v", err)
	}
	if d.state.candidate == nil || d.state.candidate.version != "version-a" {
		t.Errorf("Delivery.handleSubmit(version-a) candidate = %#v, want pending version-a", d.state.candidate)
	}
	if d.state.active == nil || d.state.active.version != "version-b" {
		t.Errorf("Delivery.handleSubmit(version-a) active = %#v, want version-b", d.state.active)
	}
}

func TestDeliveryDoesNotActivateObservedCandidateBeforePublishResultIsAccepted(t *testing.T) {
	cache := &setSnapshotErrorCache{
		SnapshotCache: cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil),
	}
	d, err := New(cache, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: "version-1",
	})

	err = d.handleSubmit(context.Background(), compiler.Result{Version: "version-1"}, false)
	if err == nil {
		t.Fatal("Delivery.handleSubmit(version-1) error = nil, want reported publication failure")
	}
	if d.state.active != nil {
		t.Errorf("Delivery.handleSubmit(version-1) active = %#v, want nil while publication failed", d.state.active)
	}
	if d.state.candidate == nil {
		t.Errorf("Delivery.handleSubmit(version-1) candidate = nil, want retained candidate")
	}
}

func TestDeliverySameCandidateRetryRechecksAcceptedVersionObservation(t *testing.T) {
	cache := &setSnapshotErrorCache{
		SnapshotCache: cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil),
	}
	d, err := New(cache, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.handleXDSEvent(context.Background(), xds.Event{
		Kind:     xds.EventStreamOpened,
		StreamID: 1,
		NodeID:   "envoy-1",
	}); err != nil {
		t.Fatalf("Delivery.handleXDSEvent(StreamOpened) error = %v", err)
	}
	d.handleAcceptedVersionObserved(xds.Event{
		Kind:            xds.EventAcceptedVersionObserved,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		AcceptedVersion: "version-1",
	})
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err == nil {
		t.Fatal("Delivery.handleSubmit(version-1) initial error = nil, want reported publication failure")
	}

	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(version-1) retry error = %v", err)
	}
	assertActiveVersion(t, d, result.Version)
}

func TestDeliveryNACKAcceptedVersionDoesNotActivateCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := compiler.Result{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.state.streams[1] = &streamState{
		nodeID:           "envoy-1",
		versions:         make(map[string]*ackProgress),
		acceptedVersions: make(map[string]string),
	}
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  result.Version,
	})
	if err := d.handleNACK(context.Background(), xds.Event{
		Kind:            xds.EventNACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		Version:         result.Version,
		AcceptedVersion: "version-old",
	}); err != nil {
		t.Fatalf("Delivery.handleNACK(%q) error = %v", result.Version, err)
	}
	if d.state.active != nil {
		t.Errorf("Delivery.handleNACK(%q) active = %#v, want nil", result.Version, d.state.active)
	}
	if got := len(d.state.streams[1].acceptedVersions); got != 0 {
		t.Errorf("Delivery.handleNACK(%q) accepted versions = %v, want empty", result.Version, d.state.streams[1].acceptedVersions)
	}
}

func TestDeliveryProcessesQueuedNACKAfterCommandContextExpires(t *testing.T) {
	snapshotCache := &failNextSetSnapshotCache{
		SnapshotCache: cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil),
	}
	d, err := New(snapshotCache, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resource := compiler.ResourceGeneration{
		Kind:       "Upstream",
		Name:       "upstream-1",
		UID:        types.UID("upstream-uid"),
		Generation: 1,
	}
	result := compiler.Result{
		Version:             "version-1",
		ResourceGenerations: []compiler.ResourceGeneration{resource},
	}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}
	d.state.streams[1] = &streamState{
		nodeID:           "envoy-1",
		versions:         make(map[string]*ackProgress),
		acceptedVersions: make(map[string]string),
	}
	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  result.Version,
	})

	commandCtx, cancelCommand := context.WithCancel(context.Background())
	cancelCommand()
	reply := make(chan error, 1)
	snapshotCache.failNext = true
	d.commands <- command{
		kind: commandXDSEvent,
		ctx:  commandCtx,
		event: xds.Event{
			Kind:            xds.EventNACK,
			StreamID:        1,
			NodeID:          "envoy-1",
			TypeURL:         resourcev3.ListenerType,
			Version:         result.Version,
			AcceptedVersion: "version-old",
		},
		reply: reply,
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- d.Run(runCtx)
	}()
	t.Cleanup(func() {
		cancelRun()
		if err := <-runDone; err != nil {
			t.Errorf("Delivery.Run() error = %v", err)
		}
	})

	select {
	case err := <-reply:
		if err == nil {
			t.Fatal("queued NACK error = nil, want fallback failure")
		}
	case <-time.After(time.Second):
		t.Fatal("queued NACK was not processed after its context expired")
	}
	if d.state.candidate != nil {
		t.Fatalf("queued NACK candidate = %#v, want nil", d.state.candidate)
	}
	status := d.Status()
	if status.LastFailure == nil || status.LastFailure.Reason != FailureDelivery {
		t.Fatalf("queued NACK last failure = %#v, want delivery failure", status.LastFailure)
	}
	if got, want := status.LastFailure.Resources, result.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("queued NACK failure resources = %v, want %v", got, want)
	}

	if err := d.HandleXDSEvent(context.Background(), xds.Event{
		Kind:            xds.EventACK,
		StreamID:        1,
		NodeID:          "envoy-1",
		TypeURL:         resourcev3.ListenerType,
		Version:         result.Version,
		AcceptedVersion: result.Version,
	}); err != nil {
		t.Fatalf("Delivery.HandleXDSEvent(late ACK) error = %v", err)
	}
	if d.state.active != nil {
		t.Errorf("Delivery.HandleXDSEvent(late ACK) active = %#v, want nil after NACK", d.state.active)
	}
}

func TestDeliveryChangeNotificationIsNonBlockingAndCoalesced(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got, want := cap(d.changes), 1; got != want {
		t.Fatalf("Delivery changes channel capacity = %d, want %d", got, want)
	}

	d.notifyChange()
	d.notifyChange()
	if got, want := len(d.changes), 1; got != want {
		t.Errorf("Delivery.notifyChange() queued notifications = %d, want %d", got, want)
	}
}

func TestDeliveryStatusReturnsIndependentPolicyTargetSlices(t *testing.T) {
	target := compiler.CompiledPolicyTarget{
		Policy: compiler.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: "rate-limit-1"},
		Target: compiler.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway-1"},
	}
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.publishStatus(Status{
		ActivePolicyTargets: []compiler.CompiledPolicyTarget{target},
		LastFailure: &Failure{
			Reason:        FailureDelivery,
			PolicyTargets: []compiler.CompiledPolicyTarget{target},
		},
	})

	status := d.Status()
	status.ActivePolicyTargets[0].Target.Name = "changed"
	status.LastFailure.PolicyTargets[0].Target.Name = "changed"
	next := d.Status()
	if got := next.ActivePolicyTargets[0].Target.Name; got != target.Target.Name {
		t.Errorf("Delivery.Status().ActivePolicyTargets mutation leaked, got target %q, want %q", got, target.Target.Name)
	}
	if got := next.LastFailure.PolicyTargets[0].Target.Name; got != target.Target.Name {
		t.Errorf("Delivery.Status().LastFailure.PolicyTargets mutation leaked, got target %q, want %q", got, target.Target.Name)
	}
}

func TestDeliveryRunNotifiesWhenOnlyActivePolicyTargetsChange(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	firstTarget := compiler.CompiledPolicyTarget{
		Policy: compiler.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: "rate-limit-1", Generation: 1},
		Target: compiler.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway-1", Generation: 1},
	}
	secondTarget := firstTarget
	secondTarget.Target.Generation = 2
	d.state.active = &publishedConfig{
		version:       "version-1",
		policyTargets: []compiler.CompiledPolicyTarget{firstTarget},
	}

	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- d.Run(runCtx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-runDone; err != nil {
			t.Errorf("Delivery.Run() error = %v", err)
		}
	})

	if err := d.Submit(context.Background(), compiler.Result{
		Version:       "version-1",
		PolicyTargets: []compiler.CompiledPolicyTarget{secondTarget},
	}); err != nil {
		t.Fatalf("Delivery.Submit(version-1) error = %v", err)
	}
	select {
	case <-d.Changes():
	case <-time.After(time.Second):
		t.Fatal("Delivery.Changes() did not report active policy target change")
	}
	if got, want := d.Status().ActivePolicyTargets, []compiler.CompiledPolicyTarget{secondTarget}; !slices.Equal(got, want) {
		t.Errorf("Delivery.Status().ActivePolicyTargets = %v, want %v", got, want)
	}
}

func TestDeliveryRunNotifiesAfterCandidateActivation(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- d.Run(runCtx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-runDone; err != nil {
			t.Errorf("Delivery.Run() error = %v", err)
		}
	})

	result := compiler.Result{
		Version: "version-1",
		ResourceGenerations: []compiler.ResourceGeneration{{
			Kind:       "Upstream",
			Name:       "upstream-1",
			UID:        types.UID("upstream-uid"),
			Generation: 1,
		}},
		PolicyTargets: []compiler.CompiledPolicyTarget{
			{
				Policy: compiler.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: "rate-limit-1"},
				Target: compiler.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway-1"},
			},
		},
	}
	if err := d.Submit(context.Background(), result); err != nil {
		t.Fatalf("Delivery.Submit(%q) error = %v", result.Version, err)
	}
	select {
	case <-d.Changes():
		t.Fatalf("Delivery.Changes() reported candidate-only change for %q", result.Version)
	default:
	}

	for _, event := range []xds.Event{
		{Kind: xds.EventStreamOpened, StreamID: 1, NodeID: "envoy-1"},
		{
			Kind:     xds.EventResponseSent,
			StreamID: 1,
			NodeID:   "envoy-1",
			TypeURL:  resourcev3.ListenerType,
			Version:  result.Version,
		},
		{
			Kind:            xds.EventACK,
			StreamID:        1,
			NodeID:          "envoy-1",
			TypeURL:         resourcev3.ListenerType,
			Version:         result.Version,
			AcceptedVersion: result.Version,
		},
	} {
		if err := d.HandleXDSEvent(context.Background(), event); err != nil {
			t.Fatalf("Delivery.HandleXDSEvent(%q) error = %v", event.Kind, err)
		}
	}

	select {
	case <-d.Changes():
	case <-time.After(time.Second):
		t.Fatalf("Delivery.Changes() did not report active resources for %q", result.Version)
	}
	if got, want := d.Status().ActiveResources, result.ResourceGenerations; !slices.Equal(got, want) {
		t.Errorf("Delivery.Status().ActiveResources = %v, want %v", got, want)
	}
	if got, want := d.Status().ActivePolicyTargets, result.PolicyTargets; !slices.Equal(got, want) {
		t.Errorf("Delivery.Status().ActivePolicyTargets = %v, want %v", got, want)
	}
}

func assertActiveVersion(t *testing.T, d *Delivery, version string) {
	t.Helper()
	if d.state.candidate != nil {
		t.Errorf("Delivery candidate = %#v, want nil after accepting version %q", d.state.candidate, version)
	}
	if d.state.active == nil || d.state.active.version != version {
		t.Errorf("Delivery active = %#v, want version %q", d.state.active, version)
	}
}
