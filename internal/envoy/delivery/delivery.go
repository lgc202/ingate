package delivery

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/xds"
)

// Delivery 串行管理 Candidate、Active 和 ACK/NACK
//
// Run 运行后，Submit、HandleXDSEvent 和 Status 可被多个 goroutine 并发调用
type Delivery struct {
	cache    cachev3.SnapshotCache
	baseline *cachev3.Snapshot
	options  Options

	commands chan command
	changes  chan struct{}
	started  chan struct{}
	done     chan struct{}
	running  atomic.Bool

	statusMu sync.RWMutex
	status   Status

	state deliveryState
}

// New 创建尚未运行的 Delivery
func New(cache cachev3.SnapshotCache, options Options) (*Delivery, error) {
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	baseline, err := newBaselineSnapshot()
	if err != nil {
		return nil, err
	}

	state := newDeliveryState()
	return &Delivery{
		cache:    cache,
		baseline: baseline,
		options:  options,
		commands: make(chan command, 1),
		changes:  make(chan struct{}, 1),
		started:  make(chan struct{}),
		done:     make(chan struct{}),
		status:   state.snapshot(),
		state:    state,
	}, nil
}

// Run 执行唯一的 Delivery 命令循环
//
// ctx 取消时 Run 停止内部 timer、关闭生命周期信号并返回 nil
func (d *Delivery) Run(ctx context.Context) error {
	if !d.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	close(d.started)
	defer close(d.done)
	defer d.stopTimers()

	for {
		select {
		case <-ctx.Done():
			return nil
		case command := <-d.commands:
			previousStatus := d.state.snapshot()
			commandCtx := ctx
			if command.ctx != nil {
				commandCtx = command.ctx
			}

			var err error
			commandErr := commandCtx.Err()
			isNACK := command.kind == commandXDSEvent && command.event.Kind == xds.EventNACK
			if commandErr != nil && !isNACK {
				err = commandErr
			} else {
				err = d.handleCommand(commandCtx, command)
			}
			status := d.state.snapshot()
			d.publishStatus(status)
			if !statusEqual(previousStatus, status) {
				d.notifyChange()
			}
			if command.reply != nil {
				command.reply <- err
			}
		}
	}
}

// Submit 发布一个通过编译的完整 Envoy 配置
func (d *Delivery) Submit(ctx context.Context, result config.CompileResult) error {
	hasErrors := result.HasErrors()
	cleanResult := config.CompileResult{
		Version:       result.Version,
		Resources:     cloneResourceGenerations(result.Resources),
		PolicyTargets: clonePolicyTargets(result.PolicyTargets),
	}
	if !hasErrors && result.Version != "" {
		cleanResult.Config = cloneConfig(result.Config)
	}
	return d.call(ctx, command{
		kind:             commandSubmit,
		result:           cleanResult,
		compileHasErrors: hasErrors,
	})
}

// CancelCandidate 取消尚未成为 Active 的配置，并恢复当前 Active 或空 Baseline
func (d *Delivery) CancelCandidate(ctx context.Context) error {
	return d.call(ctx, command{kind: commandCancelCandidate})
}

// HandleXDSEvent 将一个 xDS stream 或 ACK/NACK 事件同步交给 Delivery
//
// NACK 的排队和回滚总时间受 NACKRollbackTimeout 限制，超时错误应由 xDS 用于关闭 stream
func (d *Delivery) HandleXDSEvent(ctx context.Context, event xds.Event) error {
	if event.Kind != xds.EventNACK {
		return d.call(ctx, command{kind: commandXDSEvent, event: event})
	}

	rollbackCtx, cancel := context.WithTimeout(ctx, d.options.NACKRollbackTimeout)
	defer cancel()
	return d.call(rollbackCtx, command{kind: commandXDSEvent, event: event})
}

// Status 返回当前发布状态的独立快照
func (d *Delivery) Status() Status {
	d.statusMu.RLock()
	status := d.status
	d.statusMu.RUnlock()

	status.ActiveResources = cloneResourceGenerations(status.ActiveResources)
	status.ActivePolicyTargets = clonePolicyTargets(status.ActivePolicyTargets)
	if status.LastFailure != nil {
		failure := *status.LastFailure
		failure.Resources = cloneResourceGenerations(failure.Resources)
		failure.PolicyTargets = clonePolicyTargets(failure.PolicyTargets)
		status.LastFailure = &failure
	}
	return status
}

// Changes 返回 Delivery 声明式状态变化通知
//
// 通知通道容量为 1 且不会关闭，消费者应结合自身 context 退出并在收到通知后读取最新 Status
func (d *Delivery) Changes() <-chan struct{} {
	return d.changes
}

func normalizeOptions(options Options) (Options, error) {
	defaults := DefaultOptions()
	if options.ACKTimeout < 0 {
		return Options{}, errors.New("ACK timeout must not be negative")
	}
	if options.NACKRollbackTimeout < 0 {
		return Options{}, errors.New("NACK rollback timeout must not be negative")
	}
	if options.ACKTimeout == 0 {
		options.ACKTimeout = defaults.ACKTimeout
	}
	if options.NACKRollbackTimeout == 0 {
		options.NACKRollbackTimeout = defaults.NACKRollbackTimeout
	}
	return options, nil
}

func (d *Delivery) call(ctx context.Context, command command) error {
	reply := make(chan error, 1)
	command.ctx = ctx
	command.reply = reply

	select {
	case <-d.started:
	case <-d.done:
		return ErrStopped
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case d.commands <- command:
	case <-d.done:
		return ErrStopped
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-reply:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case <-d.done:
		select {
		case err := <-reply:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		default:
			return ErrStopped
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Delivery) enqueue(command command) {
	select {
	case d.commands <- command:
	case <-d.done:
	}
}

func (d *Delivery) publishStatus(status Status) {
	d.statusMu.Lock()
	d.status = status
	d.statusMu.Unlock()
}

func (d *Delivery) notifyChange() {
	select {
	case d.changes <- struct{}{}:
	default:
	}
}

func (d *Delivery) stopTimers() {
	if d.state.candidate != nil && d.state.candidate.timer != nil {
		d.state.candidate.timer.Stop()
	}
}

func (d *Delivery) handleCommand(ctx context.Context, command command) error {
	switch command.kind {
	case commandSubmit:
		return d.handleSubmit(ctx, command.result, command.compileHasErrors)
	case commandCancelCandidate:
		return d.handleCancelCandidate(ctx)
	case commandXDSEvent:
		return d.handleXDSEvent(ctx, command.event)
	case commandACKTimeout:
		d.handleACKTimeout(command.version, command.sequence)
		return nil
	default:
		return fmt.Errorf("unknown delivery command %d", command.kind)
	}
}

func (d *Delivery) handleSubmit(ctx context.Context, result config.CompileResult, hasErrors bool) error {
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
			failurePolicyTargets := affectedPolicyTargets(d.state.active, result.Resources, result.PolicyTargets)
			if err := d.restoreFallback(
				ctx,
				"restore active configuration after candidate cancellation",
				result.Resources,
				failurePolicyTargets,
			); err != nil {
				return err
			}
		}
		d.state.active.resources = cloneResourceGenerations(result.Resources)
		d.state.active.policyTargets = clonePolicyTargets(result.PolicyTargets)
		d.state.lastFailure = nil
		return nil
	}
	if d.state.candidate != nil && d.state.candidate.version == result.Version {
		if configsEqual(d.state.candidate.config, result.Config) {
			d.state.candidate.resources = cloneResourceGenerations(result.Resources)
			d.state.candidate.policyTargets = clonePolicyTargets(result.PolicyTargets)
			d.state.candidate.failurePolicyTargets = affectedPolicyTargets(d.state.active, result.Resources, result.PolicyTargets)
			if d.state.lastFailure != nil {
				d.state.lastFailure.Resources = cloneResourceGenerations(result.Resources)
				d.state.lastFailure.PolicyTargets = clonePolicyTargets(d.state.candidate.failurePolicyTargets)
			}
			d.activateAcceptedCandidate()
			return nil
		}
		return fmt.Errorf("%w: candidate version %q", ErrVersionConflict, result.Version)
	}

	failurePolicyTargets := affectedPolicyTargets(d.state.active, result.Resources, result.PolicyTargets)
	snapshot, err := result.Config.Snapshot(result.Version)
	if err != nil {
		d.recordFailure(FailureDelivery, result.Resources, failurePolicyTargets)
		return fmt.Errorf("build candidate snapshot %q: %w", result.Version, err)
	}
	publishErr := d.cache.SetSnapshot(ctx, xds.CacheKey, snapshot)
	if publishErr != nil && !d.cacheHasVersion(result.Version) {
		d.recordFailure(FailureDelivery, result.Resources, failurePolicyTargets)
		return fmt.Errorf("publish candidate snapshot %q: %w", result.Version, publishErr)
	}
	d.setCandidate(result, snapshot)
	if publishErr != nil {
		d.recordFailure(FailureDelivery, result.Resources, d.state.candidate.failurePolicyTargets)
		return fmt.Errorf("publish candidate snapshot %q: %w", result.Version, publishErr)
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
	return d.restoreFallback(ctx, "cancel candidate after desired configuration changed", failureResources, failurePolicyTargets)
}

func (d *Delivery) setCandidate(result config.CompileResult, snapshot *cachev3.Snapshot) {
	if d.state.candidate != nil && d.state.candidate.timer != nil {
		d.state.candidate.timer.Stop()
	}
	d.state.sequence++
	d.state.candidate = &candidateState{
		publishedConfig: publishedConfig{
			version:       result.Version,
			config:        result.Config,
			snapshot:      snapshot,
			resources:     cloneResourceGenerations(result.Resources),
			policyTargets: clonePolicyTargets(result.PolicyTargets),
		},
		sequence: d.state.sequence,
		// 保留 Active 用过的动态类型，才能通过 Candidate 的空响应确认资源删除
		requiredTypes:        transitionTypeURLs(d.state.active, result.Config),
		failurePolicyTargets: affectedPolicyTargets(d.state.active, result.Resources, result.PolicyTargets),
	}
	d.state.lastFailure = nil

	activeVersion := ""
	if d.state.active != nil {
		activeVersion = d.state.active.version
	}
	d.state.pruneProgress(activeVersion, result.Version)
}

func (d *Delivery) cacheHasVersion(version string) bool {
	snapshot, err := d.cache.GetSnapshot(xds.CacheKey)
	if err != nil {
		return false
	}
	for _, typeURL := range dynamicTypeURLs() {
		if snapshot.GetVersion(typeURL) != version {
			return false
		}
	}
	return true
}

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

func (d *Delivery) restoreFallback(
	ctx context.Context,
	operation string,
	failureResources []config.ResourceGeneration,
	failurePolicyTargets []config.ProgrammedPolicyTarget,
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

func (d *Delivery) candidateResources() []config.ResourceGeneration {
	if d.state.candidate == nil {
		return nil
	}
	return cloneResourceGenerations(d.state.candidate.resources)
}

func (d *Delivery) candidateFailurePolicyTargets() []config.ProgrammedPolicyTarget {
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
	resources []config.ResourceGeneration,
	policyTargets []config.ProgrammedPolicyTarget,
) {
	d.state.lastFailure = &Failure{
		Reason:        reason,
		Resources:     cloneResourceGenerations(resources),
		PolicyTargets: clonePolicyTargets(policyTargets),
	}
}

func statusEqual(a, b Status) bool {
	if !slices.Equal(a.ActiveResources, b.ActiveResources) {
		return false
	}
	if !slices.Equal(a.ActivePolicyTargets, b.ActivePolicyTargets) {
		return false
	}
	if a.LastFailure == nil || b.LastFailure == nil {
		return a.LastFailure == nil && b.LastFailure == nil
	}
	return a.LastFailure.Reason == b.LastFailure.Reason &&
		slices.Equal(a.LastFailure.Resources, b.LastFailure.Resources) &&
		slices.Equal(a.LastFailure.PolicyTargets, b.LastFailure.PolicyTargets)
}
