// Package status 维护声明式资源的 Accepted 状态和控制面编译状态
package status

import (
	"sync"

	"github.com/lgc202/ingate/internal/envoy/config"
)

// State 是 Runtime 对外提供的当前编译状态副本
type State struct {
	Reconciled  bool
	Diagnostics []config.Diagnostic
}

// Runtime 保存最近一次全配置域编译结果
//
// Runtime 可由 Reconciler 写入，并由状态接口并发读取。Delivery 状态不在这里缓存，
// 调用方应在读取时直接组合 Delivery.Status，避免 ACK 和超时状态变旧
type Runtime struct {
	mu          sync.RWMutex
	reconciled  bool
	diagnostics []config.Diagnostic
}

// NewRuntime 创建空的编译状态容器
func NewRuntime() *Runtime {
	return &Runtime{}
}

// UpdateDiagnostics 保存最近一次全配置域编译诊断
func (r *Runtime) UpdateDiagnostics(diagnostics []config.Diagnostic) {
	r.mu.Lock()
	r.reconciled = true
	r.diagnostics = append(r.diagnostics[:0], diagnostics...)
	r.mu.Unlock()
}

// Snapshot 返回不共享内部切片的当前状态副本
func (r *Runtime) Snapshot() State {
	r.mu.RLock()
	state := State{
		Reconciled:  r.reconciled,
		Diagnostics: append([]config.Diagnostic(nil), r.diagnostics...),
	}
	r.mu.RUnlock()
	return state
}
