package delivery

import (
	"context"
	"fmt"
	"time"
)

func (d *Delivery) handleXDSEvent(ctx context.Context, event XDSEvent) error {
	switch event.Kind {
	case EventStreamOpened:
		if _, exists := d.state.streams[event.StreamID]; !exists {
			d.state.streams[event.StreamID] = &streamState{
				nodeID:           event.NodeID,
				versions:         make(map[string]*responseProgress),
				acceptedVersions: make(map[string]string),
			}
		}
		return nil
	case EventStreamClosed:
		delete(d.state.streams, event.StreamID)
		d.activateAcceptedCandidate()
		return nil
	case EventResponseSent:
		d.handleResponseSent(event)
		return nil
	case EventAcceptedVersionObserved:
		d.handleAcceptedVersionObserved(event)
		return nil
	case EventACK:
		d.handleACK(event)
		return nil
	case EventNACK:
		return d.handleNACK(ctx, event)
	default:
		return fmt.Errorf("unknown xDS event kind %q", event.Kind)
	}
}

func (d *Delivery) handleResponseSent(event XDSEvent) {
	if d.state.candidate == nil || event.Version != d.state.candidate.version {
		return
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return
	}
	progress := stream.progress(event.Version)
	progress.sent[event.TypeURL] = true

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

func (d *Delivery) handleAcceptedVersionObserved(event XDSEvent) {
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

func (d *Delivery) handleACK(event XDSEvent) {
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
	stream.recordAccepted(event.TypeURL, event.Version)
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
	connectedStreams := 0
	for _, stream := range d.state.streams {
		if stream.nodeID == "" {
			continue
		}
		connectedStreams++
		if !stream.fullyAccepted(d.state.candidate.version, d.state.candidate.requiredTypes) {
			return
		}
	}
	if connectedStreams > 0 {
		d.activateCandidate()
	}
}

func (d *Delivery) handleNACK(ctx context.Context, event XDSEvent) error {
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

func (d *Delivery) handleACKTimeout(ctx context.Context, version string, sequence uint64) error {
	candidate := d.state.candidate
	if candidate == nil || candidate.version != version || candidate.sequence != sequence {
		return nil
	}
	resources := cloneResourceGenerations(candidate.resources)
	policyTargets := clonePolicyTargets(candidate.failurePolicyTargets)
	d.recordFailure(FailureDelivery, resources, policyTargets)
	return d.restoreFallback(
		ctx,
		fmt.Sprintf("rollback candidate %q after ACK timeout", version),
		resources,
		policyTargets,
	)
}
