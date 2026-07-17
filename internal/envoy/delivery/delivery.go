package delivery

import (
	"context"
	"errors"
	"fmt"
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
	started  chan struct{}
	done     chan struct{}
	running  atomic.Bool

	statusMu sync.RWMutex
	status   Status

	state runtimeState
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

	state := newRuntimeState()
	return &Delivery{
		cache:    cache,
		baseline: baseline,
		options:  options,
		commands: make(chan command, 1),
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
			commandCtx := ctx
			if command.ctx != nil {
				commandCtx = command.ctx
			}

			var err error
			if commandCtx.Err() != nil {
				err = commandCtx.Err()
			} else {
				err = d.handleCommand(commandCtx, command)
			}
			d.state.refreshState()
			d.publishStatus()
			if command.reply != nil {
				command.reply <- err
			}
		}
	}
}

// Submit 发布一个通过编译的完整 Envoy 配置
func (d *Delivery) Submit(ctx context.Context, result config.CompileResult) error {
	hasErrors := result.HasErrors()
	cleanResult := config.CompileResult{Version: result.Version}
	if !hasErrors && result.Version != "" {
		cleanResult.Config = cloneConfig(result.Config)
	}
	return d.call(ctx, command{
		kind:             commandSubmit,
		result:           cleanResult,
		compileHasErrors: hasErrors,
	})
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

	if status.LastNACK != nil {
		lastNACK := *status.LastNACK
		status.LastNACK = &lastNACK
	}
	return status
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

func (d *Delivery) publishStatus() {
	status := d.state.snapshot()
	d.statusMu.Lock()
	d.status = status
	d.statusMu.Unlock()
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
	if d.state.rejected[result.Version] {
		return nil
	}
	if d.state.active != nil && d.state.active.version == result.Version {
		if configsEqual(d.state.active.config, result.Config) {
			return nil
		}
		return fmt.Errorf("%w: active version %q", ErrVersionConflict, result.Version)
	}
	if d.state.candidate != nil && d.state.candidate.version == result.Version {
		if configsEqual(d.state.candidate.config, result.Config) {
			return nil
		}
		return fmt.Errorf("%w: candidate version %q", ErrVersionConflict, result.Version)
	}

	snapshot, err := result.Config.Snapshot(result.Version)
	if err != nil {
		return fmt.Errorf("build candidate snapshot %q: %w", result.Version, err)
	}
	publishErr := d.cache.SetSnapshot(ctx, xds.CacheKey, snapshot)
	if publishErr != nil && !d.cacheHasVersion(result.Version) {
		return fmt.Errorf("publish candidate snapshot %q: %w", result.Version, publishErr)
	}
	d.setCandidate(result, snapshot)
	if publishErr != nil {
		return fmt.Errorf("publish candidate snapshot %q: %w", result.Version, publishErr)
	}
	return nil
}

func (d *Delivery) setCandidate(result config.CompileResult, snapshot *cachev3.Snapshot) {
	if d.state.candidate != nil && d.state.candidate.timer != nil {
		d.state.candidate.timer.Stop()
	}
	d.state.sequence++
	d.state.candidate = &candidateState{
		publishedConfig: publishedConfig{
			version:  result.Version,
			config:   result.Config,
			snapshot: snapshot,
		},
		sequence: d.state.sequence,
		// 保留 Active 用过的动态类型，才能通过 Candidate 的空响应确认资源删除
		requiredTypes: transitionTypeURLs(d.state.active, result.Config),
	}
	d.state.ackTimedOut = false
	d.state.nackCount = 0
	d.state.rollbackError = ""
	d.state.activeNACK = false

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
				nodeID:   event.NodeID,
				versions: make(map[string]*ackProgress),
			}
		}
		return nil
	case xds.EventStreamClosed:
		delete(d.state.streams, event.StreamID)
		return nil
	case xds.EventResponseSent:
		d.handleResponseSent(event)
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
	currentCandidate := d.state.candidate != nil && event.Version == d.state.candidate.version
	currentActive := d.state.active != nil && event.Version == d.state.active.version
	if !currentCandidate && !currentActive {
		return
	}
	stream, ok := d.state.stream(event.StreamID, event.NodeID)
	if !ok {
		return
	}
	stream.progress(event.Version).sent[event.TypeURL] = true
	if !currentCandidate {
		return
	}

	candidate := d.state.candidate
	candidate.responseSeen = true
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

func (d *Delivery) handleACK(event xds.Event) {
	currentCandidate := d.state.candidate != nil && event.Version == d.state.candidate.version
	currentActive := d.state.active != nil && event.Version == d.state.active.version
	if !currentCandidate && !currentActive {
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
	if !currentCandidate || !stream.fullyACKed(event.Version, d.state.candidate.requiredTypes) {
		return
	}
	d.activateCandidate()
}

func (d *Delivery) activateCandidate() {
	candidate := d.state.candidate
	if candidate.timer != nil {
		candidate.timer.Stop()
	}

	d.state.active = &publishedConfig{
		version:  candidate.version,
		config:   candidate.config,
		snapshot: candidate.snapshot,
	}
	d.state.candidate = nil
	d.state.ackTimedOut = false
	d.state.rollbackError = ""
	d.state.activeNACK = false
	d.state.pruneProgress(candidate.version)
}

func (d *Delivery) handleNACK(ctx context.Context, event xds.Event) error {
	currentCandidate := d.state.candidate != nil && event.Version == d.state.candidate.version
	currentActive := d.state.active != nil && event.Version == d.state.active.version
	if !currentCandidate && !currentActive {
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

	message := summarizeMessage(event.ErrorMessage)
	if message == "" {
		message = fmt.Sprintf("xDS NACK code %d", event.ErrorCode)
	}
	d.state.nackCount++
	d.state.lastNACK = &NACK{
		NodeID:  stream.nodeID,
		TypeURL: event.TypeURL,
		Version: event.Version,
		Time:    time.Now().UTC(),
		Message: message,
	}
	if !currentCandidate {
		d.state.activeNACK = true
		return nil
	}

	candidate := d.state.candidate
	if candidate.timer != nil {
		candidate.timer.Stop()
	}
	d.state.rejected[candidate.version] = true
	d.state.candidate = nil
	d.state.ackTimedOut = false

	fallback := d.baseline
	activeVersion := ""
	if d.state.active != nil {
		fallback = d.state.active.snapshot
		activeVersion = d.state.active.version
	}
	d.state.pruneProgress(activeVersion)
	// SetSnapshot 必须在 NACK command reply 前完成，避免标准 server 继续从坏版本重建 watch
	if err := d.cache.SetSnapshot(ctx, xds.CacheKey, fallback); err != nil {
		d.state.rollbackError = summarizeError(err)
		return fmt.Errorf("rollback rejected candidate %q: %w", candidate.version, err)
	}
	d.state.rollbackError = ""
	return nil
}

func (d *Delivery) handleACKTimeout(version string, sequence uint64) {
	candidate := d.state.candidate
	if candidate == nil || candidate.version != version || candidate.sequence != sequence {
		return
	}
	candidate.timer = nil
	candidate.responseSeen = true
	d.state.ackTimedOut = true
}
