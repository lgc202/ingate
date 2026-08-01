package delivery

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/controller/compiler"
	"github.com/lgc202/ingate/internal/controller/xds"
)

func (d *Delivery) restoreFallback(
	ctx context.Context,
	operation string,
	failureResources []compiler.ResourceGeneration,
	failurePolicyTargets []compiler.CompiledPolicyTarget,
) error {
	if d.state.candidate != nil && d.state.candidate.timer != nil {
		d.state.candidate.timer.Stop()
	}

	fallback := d.baseline
	fallbackVersion := BaselineVersion
	if d.state.active != nil {
		fallback = d.state.active.snapshot
		fallbackVersion = d.state.active.version
	}

	// SetSnapshot 必须在 command reply 前完成，避免标准 server 继续从已撤回版本重建 watch
	err := d.cache.SetSnapshot(ctx, xds.CacheKey, fallback)
	d.state.candidate = nil
	d.state.pruneProgress(fallbackVersion)
	d.clearAcceptedVersions()
	if err != nil {
		d.recordFailure(FailureDelivery, failureResources, failurePolicyTargets)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func (d *Delivery) candidateResources() []compiler.ResourceGeneration {
	if d.state.candidate == nil {
		return nil
	}
	return cloneResourceGenerations(d.state.candidate.resources)
}

func (d *Delivery) candidateFailurePolicyTargets() []compiler.CompiledPolicyTarget {
	if d.state.candidate == nil {
		return nil
	}
	return clonePolicyTargets(d.state.candidate.failurePolicyTargets)
}

func (d *Delivery) clearAcceptedVersions() {
	for _, stream := range d.state.streams {
		clear(stream.acceptedVersions)
	}
}

func (d *Delivery) recordFailure(
	reason FailureReason,
	resources []compiler.ResourceGeneration,
	policyTargets []compiler.CompiledPolicyTarget,
) {
	d.state.lastFailure = &Failure{
		Reason:        reason,
		Resources:     cloneResourceGenerations(resources),
		PolicyTargets: clonePolicyTargets(policyTargets),
	}
}
