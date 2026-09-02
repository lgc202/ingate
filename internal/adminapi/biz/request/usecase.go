// Package request 提供控制台请求记录查询用例。
package request

import (
	"context"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
)

var (
	// ErrNotFound 表示请求记录不存在或已经超过明细保留期。
	ErrNotFound = apperror.RequestRecordNotFound()
	// ErrUnavailable 表示请求分析组件当前无法提供查询。
	ErrUnavailable = apperror.DependencyUnavailable("请求记录服务暂时不可用，请稍后重试", nil)
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
	return apperror.DependencyUnavailable("请求记录服务暂时不可用，请稍后重试", cause)
}
