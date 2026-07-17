// Package delivery 管理 Envoy 配置从 Candidate 到 Active 和 Last Good 的完整发布生命周期
package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/lgc202/ingate/internal/envoy/lastgood"
)

// State 表示当前 Envoy 配置发布状态
type State string

const (
	// StateNoConfig 表示 Cache 中只有空 Baseline
	StateNoConfig State = "NoConfig"
	// StateWaitingForEnvoy 表示 Candidate 已发布但尚未向 Envoy 发送响应
	StateWaitingForEnvoy State = "WaitingForEnvoy"
	// StateWaitingForACK 表示 Candidate 已发送并正在等待完整 ACK
	StateWaitingForACK State = "WaitingForACK"
	// StateActive 表示当前配置已经被至少一个 Envoy 完整接受
	StateActive State = "Active"
	// StateDegraded 表示仍可继续服务，但恢复、持久化或实例收敛存在异常
	StateDegraded State = "Degraded"
)

const (
	// DefaultACKTimeout 是 Candidate 首次发送后的默认 ACK 等待时间
	DefaultACKTimeout = 30 * time.Second
	// DefaultNACKRollbackTimeout 是同步恢复旧配置的默认最长等待时间
	DefaultNACKRollbackTimeout = 3 * time.Second
	// DefaultLastGoodRetryInterval 是 Last Good 持久化失败后的默认重试间隔
	DefaultLastGoodRetryInterval = 5 * time.Second
)

var (
	// ErrAlreadyRunning 表示同一个 Delivery 被重复运行
	ErrAlreadyRunning = errors.New("delivery is already running")
	// ErrStopped 表示 Delivery 的命令循环已经退出
	ErrStopped = errors.New("delivery is stopped")
	// ErrNotRestored 表示尚未完成 Last Good 或 Baseline 初始化
	ErrNotRestored = errors.New("delivery is not restored")
	// ErrInvalidCompileResult 表示编译结果不能作为 Candidate 发布
	ErrInvalidCompileResult = errors.New("compile result is not publishable")
	// ErrVersionConflict 表示同一版本对应了不同 Envoy 配置内容
	ErrVersionConflict = errors.New("config version conflicts with existing content")
)

// Options 配置 Delivery 内部超时和重试节奏
//
// 零值字段使用对应默认值，负值会被 New 拒绝
type Options struct {
	ACKTimeout            time.Duration
	NACKRollbackTimeout   time.Duration
	LastGoodRetryInterval time.Duration
}

// DefaultOptions 返回生产环境默认参数
func DefaultOptions() Options {
	return Options{
		ACKTimeout:            DefaultACKTimeout,
		NACKRollbackTimeout:   DefaultNACKRollbackTimeout,
		LastGoodRetryInterval: DefaultLastGoodRetryInterval,
	}
}

// ACKSummary 汇总当前 Candidate 或 Active 在单个最佳进度实例上的 ACK 数量
//
// Received 不会跨 Envoy 合并，只有同一实例完成全部 Required 才能激活 Candidate
type ACKSummary struct {
	Required int `json:"required"`
	Received int `json:"received"`
}

// NACKSummary 汇总当前发布周期收到的有效 NACK
type NACKSummary struct {
	Count int `json:"count"`
}

// NACK 记录最近一次影响当前 Candidate 或 Active 的 Envoy 拒绝信息
type NACK struct {
	NodeID  string    `json:"nodeID"`
	TypeURL string    `json:"typeURL"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// Status 是 Delivery 对外提供的并发安全状态快照
type Status struct {
	CandidateVersion    string      `json:"candidateVersion"`
	ActiveVersion       string      `json:"activeVersion"`
	LastGoodVersion     string      `json:"lastGoodVersion"`
	ConfigReady         bool        `json:"configReady"`
	State               State       `json:"deliveryState"`
	ConnectedEnvoys     int         `json:"connectedEnvoys"`
	ACKs                ACKSummary  `json:"acks"`
	NACKs               NACKSummary `json:"nacks"`
	LastNACK            *NACK       `json:"lastNack,omitempty"`
	ACKTimedOut         bool        `json:"ackTimedOut"`
	PersistenceError    string      `json:"persistenceError,omitempty"`
	PersistenceRetrying bool        `json:"persistenceRetrying"`
	RollbackError       string      `json:"rollbackError,omitempty"`
}

// LastGoodStore 是 Delivery 使用的持久化边界
type LastGoodStore interface {
	Load(context.Context) (lastgood.Record, error)
	Save(context.Context, lastgood.Record) error
}

var _ LastGoodStore = (*lastgood.Store)(nil)
