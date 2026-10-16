package handler

import (
	"context"
	"errors"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ adminv1.AccountServiceServer     = (*AccountHandler)(nil)
	_ adminv1.AccountServiceHTTPServer = (*AccountHandler)(nil)
)

// AccountHandler 实现管理员账户的 HTTP 和 gRPC 接口。
type AccountHandler struct {
	adminv1.UnimplementedAccountServiceServer

	service *accountservice.Service
}

// NewAccount 创建管理员账户协议入口，复用注入的业务服务。
func NewAccount(service *accountservice.Service) *AccountHandler {
	return &AccountHandler{service: service}
}

// Login 校验密码并创建登录态。
func (h *AccountHandler) Login(ctx context.Context, req *adminv1.LoginRequest) (*adminv1.LoginReply, error) {
	result, err := h.service.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, toAccountError(err)
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
func (h *AccountHandler) Logout(ctx context.Context, _ *adminv1.LogoutRequest) (*adminv1.LogoutReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toAccountError(accountservice.ErrSessionRequired)
	}

	if err := h.service.Logout(ctx, session.ID); err != nil {
		return nil, toAccountError(err)
	}

	return &adminv1.LogoutReply{}, nil
}

// GetCurrentAccount 读取当前账户。
func (h *AccountHandler) GetCurrentAccount(
	ctx context.Context,
	_ *adminv1.GetCurrentAccountRequest,
) (*adminv1.GetCurrentAccountReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toAccountError(accountservice.ErrSessionRequired)
	}

	account, err := h.service.Get(ctx, session.AccountID)
	if err != nil {
		return nil, toAccountError(err)
	}

	return &adminv1.GetCurrentAccountReply{Account: accountToProto(account)}, nil
}

// ChangePassword 校验当前密码并使账户全部旧登录态失效。
func (h *AccountHandler) ChangePassword(
	ctx context.Context,
	req *adminv1.ChangePasswordRequest,
) (*adminv1.ChangePasswordReply, error) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		return nil, toAccountError(accountservice.ErrSessionRequired)
	}

	err := h.service.ChangePassword(
		ctx,
		session.AccountID,
		req.GetCurrentPassword(),
		req.GetNewPassword(),
	)
	if err != nil {
		if errors.Is(err, accountservice.ErrInvalidCredentials) {
			return nil, errno.ErrCurrentPasswordInvalid.WithCause(err)
		}

		return nil, toAccountError(err)
	}

	return &adminv1.ChangePasswordReply{}, nil
}

func accountToProto(account domain.Account) *adminv1.Account {
	return &adminv1.Account{Id: account.ID, Username: account.Username}
}

func toAccountError(err error) error {
	switch {
	case errors.Is(err, accountservice.ErrInvalidCredentials):
		return errno.ErrAuthInvalidCredentials.WithCause(err)
	case errors.Is(err, accountservice.ErrPasswordUnchanged):
		return errno.ErrPasswordUnchanged.WithCause(err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return errno.Fallback(err)
	case errors.Is(err, accountservice.ErrSessionRequired):
		return errno.ErrAuthSessionRequired.WithCause(err)
	case errors.Is(err, accountservice.ErrUnavailable):
		return errno.ErrAuthUnavailable.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
