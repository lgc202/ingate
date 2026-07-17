package delivery

import (
	"context"
	"testing"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/xds"
)

func TestDeliveryAllowsNACKedVersionToBeSubmittedAgain(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := config.CompileResult{Version: "version-1"}
	if err := d.handleSubmit(context.Background(), result, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", result.Version, err)
	}

	d.state.streams[1] = &streamState{
		nodeID:   "envoy-1",
		versions: make(map[string]*ackProgress),
	}
	d.state.streams[1].progress(result.Version).sent[resourcev3.ListenerType] = true
	if err := d.handleNACK(context.Background(), xds.Event{
		Kind:         xds.EventNACK,
		StreamID:     1,
		NodeID:       "envoy-1",
		TypeURL:      resourcev3.ListenerType,
		Version:      result.Version,
		ErrorMessage: "rejected",
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

func TestRuntimeStateACKTimeoutIsDegraded(t *testing.T) {
	state := newRuntimeState()
	state.candidate = &candidateState{
		publishedConfig: publishedConfig{version: "version-1"},
		sequence:        1,
		responseSeen:    true,
	}
	state.ackTimedOut = true
	state.refreshState()

	if state.state != StateDegraded {
		t.Errorf("runtimeState.refreshState() state = %q, want %q", state.state, StateDegraded)
	}
}

func TestDeliveryCancelCandidateRestoresBaseline(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result := config.CompileResult{Version: "version-1"}
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

func TestDeliverySubmittingActiveVersionCancelsCandidate(t *testing.T) {
	d, err := New(cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil), Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	active := config.CompileResult{Version: "version-active"}
	if err := d.handleSubmit(context.Background(), active, false); err != nil {
		t.Fatalf("Delivery.handleSubmit(%q) error = %v", active.Version, err)
	}
	d.activateCandidate()

	candidate := config.CompileResult{Version: "version-candidate"}
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

func TestDeliveryActiveNACKClearsAfterFullACK(t *testing.T) {
	d := &Delivery{state: newRuntimeState()}
	d.state.active = &publishedConfig{
		version: "version-active",
		config:  config.Config{},
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
		Version:  d.state.active.version,
	})
	if err := d.handleNACK(context.Background(), xds.Event{
		Kind:         xds.EventNACK,
		StreamID:     1,
		NodeID:       "envoy-1",
		TypeURL:      resourcev3.ListenerType,
		Version:      d.state.active.version,
		ErrorMessage: "rejected",
	}); err != nil {
		t.Fatalf("Delivery.handleNACK(%q) error = %v", d.state.active.version, err)
	}
	d.state.refreshState()
	if d.state.state != StateDegraded {
		t.Fatalf("Delivery state after active NACK = %q, want %q", d.state.state, StateDegraded)
	}

	d.handleResponseSent(xds.Event{
		Kind:     xds.EventResponseSent,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  d.state.active.version,
	})
	d.handleACK(xds.Event{
		Kind:     xds.EventACK,
		StreamID: 1,
		NodeID:   "envoy-1",
		TypeURL:  resourcev3.ListenerType,
		Version:  d.state.active.version,
	})
	d.state.refreshState()
	if d.state.state != StateActive {
		t.Errorf("Delivery state after active full ACK = %q, want %q", d.state.state, StateActive)
	}
}
