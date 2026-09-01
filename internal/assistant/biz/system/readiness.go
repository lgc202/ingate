// Package system 汇总运维助手进程及其依赖的运行状态。
package system

import (
	"context"
	"sync"
	"time"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

const (
	// StatusReady 表示组件当前可以承载请求。
	StatusReady Status = iota + 1
	// StatusUnavailable 表示组件当前不可用。
	StatusUnavailable
)
const (
	// ComponentMySQL 是持久存储组件的稳定名称。
	ComponentMySQL = "mysql"
	// ComponentTemporal 是工作流服务组件的稳定名称。
	ComponentTemporal = "temporal"
	// ComponentModel 是模型服务组件的稳定名称。
	ComponentModel = "model"
	// ComponentWorker 是进程内 Temporal Worker 的稳定名称。
	ComponentWorker = "worker"
)

// Status 表示一个必需组件的就绪状态。
type Status uint8

// DatabaseChecker 定义持久存储就绪检查边界。
type DatabaseChecker interface {
	Check(context.Context) error
}

// WorkflowChecker 定义工作流服务就绪检查边界。
type WorkflowChecker interface {
	Check(context.Context) error
}

// ModelChecker 定义模型服务就绪检查边界。
type ModelChecker interface {
	Check(context.Context) error
}

// WorkerChecker 定义进程内 Worker 就绪检查边界。
type WorkerChecker interface {
	Check(context.Context) error
}

// Component 是单个必需组件的状态。
type Component struct {
	Name   string
	Status Status
}

// Report 是一次完整就绪检查的稳定结果。
type Report struct {
	Status     Status
	Components []Component
}

type dependency struct {
	name  string
	check func(context.Context) error
}

// Usecase 执行运维助手的完整就绪检查。
type Usecase struct {
	timeout      time.Duration
	dependencies []dependency
}

// NewUsecase 创建使用同一总期限并行检查依赖的用例。
func NewUsecase(
	config *conf.Server,
	database DatabaseChecker,
	workflow WorkflowChecker,
	model ModelChecker,
	worker WorkerChecker,
) *Usecase {
	return &Usecase{
		timeout: config.GetHttp().GetReadinessTimeout().AsDuration(),
		dependencies: []dependency{
			{name: ComponentMySQL, check: database.Check},
			{name: ComponentTemporal, check: workflow.Check},
			{name: ComponentModel, check: model.Check},
			{name: ComponentWorker, check: worker.Check},
		},
	}
}

// Check 返回所有依赖的状态；单个失败不会掩盖其他检查结果。
func (u *Usecase) Check(ctx context.Context) Report {
	checkCtx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	report := Report{
		Status:     StatusReady,
		Components: make([]Component, len(u.dependencies)),
	}
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(u.dependencies))
	for index, item := range u.dependencies {
		go func() {
			defer waitGroup.Done()
			status := StatusReady
			if err := item.check(checkCtx); err != nil {
				status = StatusUnavailable
			}
			report.Components[index] = Component{Name: item.name, Status: status}
		}()
	}
	waitGroup.Wait()
	for _, component := range report.Components {
		if component.Status == StatusUnavailable {
			report.Status = StatusUnavailable
			break
		}
	}
	return report
}
