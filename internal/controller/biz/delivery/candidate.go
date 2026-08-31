package delivery

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

func (d *Delivery) handleSubmit(ctx context.Context, result compiler.Result, hasErrors bool) error {
	if hasErrors {
		return fmt.Errorf("%w: compiler returned error diagnostics", ErrInvalidCompileResult)
	}
	if result.Version == "" {
		return fmt.Errorf("%w: version is empty", ErrInvalidCompileResult)
	}
	if d.state.active != nil && d.state.active.version == result.Version {
		if !configsEqual(d.state.active.config, result.Config) {
			return fmt.Errorf("%w: active version %q", ErrVersionConflict, result.Version)
		}
		needsRestore := d.state.candidate != nil ||
			(d.state.lastFailure != nil && d.state.lastFailure.Reason == FailureDelivery)
		if needsRestore {
			failurePolicyTargets := affectedPolicyTargets(d.state.active, result.ResourceGenerations, result.PolicyTargets)
			if err := d.restoreFallback(
				ctx,
				"restore active configuration after candidate cancellation",
				result.ResourceGenerations,
				failurePolicyTargets,
			); err != nil {
				return err
			}
		}
		d.state.active.resources = cloneResourceGenerations(result.ResourceGenerations)
		d.state.active.policyTargets = clonePolicyTargets(result.PolicyTargets)
		d.state.lastFailure = nil
		return nil
	}
	if d.state.candidate != nil && d.state.candidate.version == result.Version {
		if configsEqual(d.state.candidate.config, result.Config) {
			d.state.candidate.resources = cloneResourceGenerations(result.ResourceGenerations)
			d.state.candidate.policyTargets = clonePolicyTargets(result.PolicyTargets)
			d.state.candidate.failurePolicyTargets = affectedPolicyTargets(
				d.state.active,
				result.ResourceGenerations,
				result.PolicyTargets,
			)
			if d.state.lastFailure != nil {
				d.state.lastFailure.Resources = cloneResourceGenerations(result.ResourceGenerations)
				d.state.lastFailure.PolicyTargets = clonePolicyTargets(d.state.candidate.failurePolicyTargets)
			}
			d.activateAcceptedCandidate()
			return nil
		}
		return fmt.Errorf("%w: candidate version %q", ErrVersionConflict, result.Version)
	}

	failurePolicyTargets := affectedPolicyTargets(d.state.active, result.ResourceGenerations, result.PolicyTargets)
	publishErr := d.publisher.Publish(ctx, result.Version, result.Config)
	if publishErr != nil && !d.publisher.HasVersion(result.Version) {
		d.recordFailure(FailureDelivery, result.ResourceGenerations, failurePolicyTargets)
		return fmt.Errorf("publish candidate configuration %q: %w", result.Version, publishErr)
	}
	d.setCandidate(result)
	if publishErr != nil {
		d.recordFailure(FailureDelivery, result.ResourceGenerations, d.state.candidate.failurePolicyTargets)
		return fmt.Errorf("publish candidate configuration %q: %w", result.Version, publishErr)
	}
	d.activateAcceptedCandidate()
	return nil
}

func (d *Delivery) handleCancelCandidate(ctx context.Context) error {
	if d.state.candidate == nil && (d.state.lastFailure == nil || d.state.lastFailure.Reason != FailureDelivery) {
		d.state.lastFailure = nil
		return nil
	}
	failureResources := d.candidateResources()
	failurePolicyTargets := d.candidateFailurePolicyTargets()
	if len(failureResources) == 0 && d.state.lastFailure != nil {
		failureResources = cloneResourceGenerations(d.state.lastFailure.Resources)
		failurePolicyTargets = clonePolicyTargets(d.state.lastFailure.PolicyTargets)
	}
	d.state.lastFailure = nil
	return d.restoreFallback(
		ctx,
		"cancel candidate after desired configuration changed",
		failureResources,
		failurePolicyTargets,
	)
}

func (d *Delivery) setCandidate(result compiler.Result) {
	if d.state.candidate != nil && d.state.candidate.timer != nil {
		d.state.candidate.timer.Stop()
	}
	d.state.sequence++
	d.state.candidate = &candidateState{
		publishedConfig: publishedConfig{
			version:       result.Version,
			config:        result.Config,
			resources:     cloneResourceGenerations(result.ResourceGenerations),
			policyTargets: clonePolicyTargets(result.PolicyTargets),
		},
		sequence: d.state.sequence,
		// 保留 Active 用过的动态类型，才能通过 Candidate 的空响应确认资源删除
		requiredTypes:        transitionTypeURLs(d.state.active, result.Config),
		failurePolicyTargets: affectedPolicyTargets(d.state.active, result.ResourceGenerations, result.PolicyTargets),
	}
	d.state.lastFailure = nil

	activeVersion := ""
	if d.state.active != nil {
		activeVersion = d.state.active.version
	}
	d.state.pruneProgress(activeVersion, result.Version)
}
