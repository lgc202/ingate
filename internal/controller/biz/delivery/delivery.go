package delivery

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

// Publisher 将完整 Envoy 配置发布到数据面配置通道
//
// Delivery 只关心版本是否已发布，Snapshot Cache 等 xDS 实现细节由 server 层承担
type Publisher interface {
	Publish(context.Context, string, compiler.EnvoyConfig) error
	HasVersion(string) bool
}

// Delivery 串行管理 Candidate、Active 和 ACK/NACK
//
// Start 运行后，Submit、HandleXDSEvent 和 Status 可被多个 goroutine 并发调用
type Delivery struct {
	publisher Publisher
	options   Options

	commands chan command
	changes  chan struct{}
	started  chan struct{}
	done     chan struct{}
	running  atomic.Bool
	cancel   context.CancelFunc

	statusMu sync.RWMutex
	status   Status

	state deliveryState
}

// New 创建尚未运行的 Delivery
func New(publisher Publisher, options Options) (*Delivery, error) {
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	state := newDeliveryState()
	return &Delivery{
		publisher: publisher,
		options:   options,
		commands:  make(chan command, 1),
		changes:   make(chan struct{}, 1),
		started:   make(chan struct{}),
		done:      make(chan struct{}),
		status:    state.status(),
		state:     state,
	}, nil
}

// Start 执行唯一的 Delivery 命令循环
//
// Kratos 在独立 goroutine 中调用 Start，并在停止时调用 Stop
func (d *Delivery) Start(ctx context.Context) error {
	if !d.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	defer cancel()

	close(d.started)
	defer close(d.done)
	defer d.stopTimers()

	for {
		select {
		case <-runCtx.Done():
			return nil
		case command := <-d.commands:
			previousStatus := d.state.status()
			commandCtx := runCtx
			if command.ctx != nil {
				commandCtx = command.ctx
			}

			var err error
			commandErr := commandCtx.Err()
			isNACK := command.kind == commandXDSEvent && command.event.Kind == EventNACK
			if commandErr != nil && !isNACK {
				err = commandErr
			} else {
				err = d.handleCommand(commandCtx, command)
			}
			status := d.state.status()
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

// Stop 停止配置发布循环并等待正在处理的命令退出
func (d *Delivery) Stop(ctx context.Context) error {
	select {
	case <-d.started:
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	d.cancel()

	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready 返回配置发布循环是否正在运行
func (d *Delivery) Ready() bool {
	select {
	case <-d.started:
	default:
		return false
	}
	select {
	case <-d.done:
		return false
	default:
		return true
	}
}

// Submit 发布一个通过编译的完整 Envoy 配置
func (d *Delivery) Submit(ctx context.Context, result compiler.Result) error {
	hasErrors := result.HasErrors()
	cleanResult := compiler.Result{
		Version:             result.Version,
		ResourceGenerations: cloneResourceGenerations(result.ResourceGenerations),
		PolicyTargets:       clonePolicyTargets(result.PolicyTargets),
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
func (d *Delivery) HandleXDSEvent(ctx context.Context, event XDSEvent) error {
	if event.Kind != EventNACK {
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
