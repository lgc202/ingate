package account

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/domain"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ adminv1.AccountServiceServer     = (*Handler)(nil)
	_ adminv1.AccountServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现管理员账户的 HTTP 和 gRPC 接口。
type Handler struct {
	adminv1.UnimplementedAccountServiceServer

	service *accountservice.Service
}

// New 创建账户 Handler。
func New(service *accountservice.Service) *Handler {
	return &Handler{service: service}
}

// Login 校验密码并创建登录态。
func (h *Handler) Login(ctx context.Context, req *adminv1.LoginRequest) (*adminv1.LoginReply, error) {
	result, err := h.service.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}
	if response, ok := kratoshttp.ResponseWriterFromServerContext(ctx); ok {
		response.Header().Set("Cache-Control", "no-store")
	}

	return &adminv1.LoginReply{
		SessionToken: result.Token,
		ExpireTime:   timestamppb.New(result.ExpireTime),
		Account:      accountToProto(result.Account),
	}, nil
}

// Logout 撤销当前登录态。
func (h *Handler) Logout(ctx context.Context, _ *adminv1.LogoutRequest) (*adminv1.LogoutReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toTransportError(ctx, accountservice.ErrSessionRequired)
	}
	if err := h.service.Logout(ctx, session.ID); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.LogoutReply{}, nil
}

// GetCurrentAccount 读取当前账户。
func (h *Handler) GetCurrentAccount(
	ctx context.Context,
	_ *adminv1.GetCurrentAccountRequest,
) (*adminv1.GetCurrentAccountReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toTransportError(ctx, accountservice.ErrSessionRequired)
	}

	account, err := h.service.Get(ctx, session.AccountID)
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetCurrentAccountReply{Account: accountToProto(account)}, nil
}

// ChangePassword 校验当前密码并使账户全部旧登录态失效。
func (h *Handler) ChangePassword(
	ctx context.Context,
	req *adminv1.ChangePasswordRequest,
) (*adminv1.ChangePasswordReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toTransportError(ctx, accountservice.ErrSessionRequired)
	}

	err := h.service.ChangePassword(
		ctx,
		session.AccountID,
		req.GetCurrentPassword(),
		req.GetNewPassword(),
	)
	if err != nil {
		if errors.Is(err, accountservice.ErrInvalidCredentials) {
			return nil, kratoserrors.BadRequest("CURRENT_PASSWORD_INVALID", "当前密码错误").WithCause(err)
		}
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.ChangePasswordReply{}, nil
}

func accountToProto(account domain.Account) *adminv1.Account {
	return &adminv1.Account{Id: account.ID, Username: account.Username}
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountservice.ErrInvalidCredentials):
		return kratoserrors.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误").WithCause(err)
	case errors.Is(err, accountservice.ErrPasswordUnchanged):
		return kratoserrors.BadRequest("PASSWORD_UNCHANGED", "新密码不能与当前密码相同").WithCause(err)
	case errors.Is(err, accountservice.ErrSessionRequired), errors.Is(err, accountservice.ErrUnavailable):
		return middleware.ToAuthenticationError(ctx, err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
