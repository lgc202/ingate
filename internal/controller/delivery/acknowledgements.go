package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/controller/xds"
)

func (d *Delivery) handleXDSEvent(ctx context.Context, event xds.Event) error {
	switch event.Kind {
	case xds.EventStreamOpened:
		if _, exists := d.state.streams[event.StreamID]; !exists {
			d.state.streams[event.StreamID] = &streamState{
				nodeID:           event.NodeID,
				versions:         make(map[string]*ackProgress),
				acceptedVersions: make(map[string]string),
			}
		}
		return nil
	case xds.EventStreamClosed:
		delete(d.state.streams, event.StreamID)
		return nil
	case xds.EventResponseSent:
		d.handleResponseSent(event)
		return nil
	case xds.EventAcceptedVersionObserved:
		d.handleAcceptedVersionObserved(event)
		return nil
	case xds.EventACK:
		d.handleACK(event)
		return nil
	case xds.EventNACK:
		return d.handleNACK(ctx, event)
	default:
		return fmt.Errorf("unknown xDS event kind %q", event.Kind)
	}
}

func (d *Delivery) handleResponseSent(event xds.Event) {
	if d.state.candidate == nil || event.Version != d.state.candidate.version {
		return
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return
	}
	progress := stream.progress(event.Version)
	progress.sent[event.TypeURL] = true
	progress.acked[event.TypeURL] = false

	candidate := d.state.candidate
	if candidate.timer == nil {
		version := candidate.version
		sequence := candidate.sequence
		candidate.timer = time.AfterFunc(d.options.ACKTimeout, func() {
			d.enqueue(command{
				kind:     commandACKTimeout,
				version:  version,
				sequence: sequence,
			})
		})
	}
}

func (d *Delivery) handleAcceptedVersionObserved(event xds.Event) {
	if event.TypeURL == "" || event.AcceptedVersion == "" {
		return
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return
	}
	stream.recordAccepted(event.TypeURL, event.AcceptedVersion)
	d.activateAcceptedCandidate()
}

func (d *Delivery) handleACK(event xds.Event) {
	if d.state.candidate == nil || event.Version != d.state.candidate.version {
		return
	}
	if event.AcceptedVersion != event.Version {
		return
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return
	}
	progress := stream.versions[event.Version]
	if progress == nil || !progress.sent[event.TypeURL] {
		return
	}
	progress.acked[event.TypeURL] = true
	stream.recordAccepted(event.TypeURL, event.Version)
	if stream.fullyACKed(event.Version, d.state.candidate.requiredTypes) {
		d.activateCandidate()
		return
	}
	d.activateAcceptedCandidate()
}

func (d *Delivery) activateCandidate() {
	candidate := d.state.candidate
	if candidate.timer != nil {
		candidate.timer.Stop()
	}

	d.state.active = &publishedConfig{
		version:       candidate.version,
		config:        candidate.config,
		snapshot:      candidate.snapshot,
		resources:     candidate.resources,
		policyTargets: candidate.policyTargets,
	}
	d.state.candidate = nil
	d.state.lastFailure = nil
	d.state.pruneProgress(candidate.version)
}

func (d *Delivery) activateAcceptedCandidate() {
	if d.state.candidate == nil {
		return
	}
	for _, stream := range d.state.streams {
		if stream.fullyAccepted(d.state.candidate.version, d.state.candidate.requiredTypes) {
			d.activateCandidate()
			return
		}
	}
}

func (d *Delivery) handleNACK(ctx context.Context, event xds.Event) error {
	if d.state.candidate == nil || event.Version != d.state.candidate.version {
		return nil
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return nil
	}
	progress := stream.versions[event.Version]
	if progress == nil || !progress.sent[event.TypeURL] {
		return nil
	}

	progress.acked[event.TypeURL] = false
	resources := cloneResourceGenerations(d.state.candidate.resources)
	policyTargets := clonePolicyTargets(d.state.candidate.failurePolicyTargets)
	d.recordFailure(FailureRejected, resources, policyTargets)
	if err := d.restoreFallback(
		ctx,
		fmt.Sprintf("rollback rejected candidate %q", event.Version),
		resources,
		policyTargets,
	); err != nil {
		return err
	}
	return nil
}

func (d *Delivery) handleACKTimeout(version string, sequence uint64) {
	candidate := d.state.candidate
	if candidate == nil || candidate.version != version || candidate.sequence != sequence {
		return
	}
	d.recordFailure(FailureDelivery, candidate.resources, candidate.failurePolicyTargets)
}
