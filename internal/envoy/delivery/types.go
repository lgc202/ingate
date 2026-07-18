// Package delivery 管理 Envoy 配置从 Candidate 到 Active 的发布生命周期
package delivery

import (
	"errors"
	"time"

	"github.com/lgc202/ingate/internal/envoy/config"
)

const (
	// DefaultACKTimeout 是 Candidate 首次发送后的默认 ACK 等待时间
	DefaultACKTimeout = 30 * time.Second
	// DefaultNACKRollbackTimeout 是同步恢复旧配置的默认最长等待时间
	DefaultNACKRollbackTimeout = 3 * time.Second
)

var (
	// ErrAlreadyRunning 表示同一个 Delivery 被重复运行
	ErrAlreadyRunning = errors.New("delivery is already running")
	// ErrStopped 表示 Delivery 的命令循环已经退出
	ErrStopped = errors.New("delivery is stopped")
	// ErrInvalidCompileResult 表示编译结果不能作为 Candidate 发布
	ErrInvalidCompileResult = errors.New("compile result is not publishable")
	// ErrVersionConflict 表示同一版本对应了不同 Envoy 配置内容
	ErrVersionConflict = errors.New("config version conflicts with existing content")
)

// Options 配置 Delivery 内部超时参数
//
// 零值字段使用对应默认值，负值会被 New 拒绝
type Options struct {
	ACKTimeout          time.Duration
	NACKRollbackTimeout time.Duration
}

// DefaultOptions 返回生产环境默认参数
func DefaultOptions() Options {
	return Options{
		ACKTimeout:          DefaultACKTimeout,
		NACKRollbackTimeout: DefaultNACKRollbackTimeout,
	}
}

// FailureReason 表示最近一次配置发布失败的稳定分类
type FailureReason string

const (
	// FailureRejected 表示配置被网关实例拒绝
	FailureRejected FailureReason = "Rejected"
	// FailureDelivery 表示配置发布或回退过程失败
	FailureDelivery FailureReason = "DeliveryFailed"
)

// Failure 记录最近一次发布失败所对应的资源版本
type Failure struct {
	Reason        FailureReason
	Resources     []config.ResourceGeneration
	PolicyTargets []config.ProgrammedPolicyTarget
}

// Status 是 Delivery 对外提供的并发安全状态快照
type Status struct {
	ActiveResources     []config.ResourceGeneration
	ActivePolicyTargets []config.ProgrammedPolicyTarget
	LastFailure         *Failure
}
