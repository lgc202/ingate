// Package request 提供控制台请求记录查询用例。
package request

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

var (
	// ErrNotFound 表示请求记录不存在或已经超过明细保留期。
	ErrNotFound = errors.NotFound(
		adminv1.ErrorReason_REQUEST_RECORD_NOT_FOUND.String(),
		"请求记录不存在或已超过明细保留期",
	)
	// ErrUnavailable 表示请求分析组件当前无法提供查询。
	ErrUnavailable = errors.ServiceUnavailable(
		adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(),
		"请求记录服务暂时不可用，请稍后重试",
	)
)

// Reader 定义请求记录用例所需的分页和明细查询能力。
type Reader interface {
	List(ctx context.Context, options ListOptions) (Page, error)
	Get(ctx context.Context, recordID string, startedAt time.Time) (*Record, error)
}

// Usecase 提供不依赖 Analytics gRPC 协议的请求记录查询。
type Usecase struct {
	reader Reader
}

// NewUsecase 创建请求记录用例。
func NewUsecase(reader Reader) *Usecase {
	return &Usecase{reader: reader}
}

// List 按时间倒序查询请求记录。
func (uc *Usecase) List(ctx context.Context, options ListOptions) (Page, error) {
	return uc.reader.List(ctx, options)
}

// Get 查询单次请求记录。
func (uc *Usecase) Get(
	ctx context.Context,
	recordID string,
	startedAt time.Time,
) (*Record, error) {
	return uc.reader.Get(ctx, recordID, startedAt)
}

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义。
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}
