package repository

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	// ErrInvalid 表示资源未通过 API Server 校验。
	ErrInvalid = errors.New("invalid resource")
	// ErrAlreadyExists 表示同名资源已存在。
	ErrAlreadyExists = errors.New("resource already exists")
	// ErrNotFound 表示指定资源不存在。
	ErrNotFound = errors.New("resource not found")
	// ErrConflict 表示资源版本已发生变化。
	ErrConflict = errors.New("resource version conflict")
)

func translateResourceError(err error) error {
	switch {
	case apierrors.IsBadRequest(err), apierrors.IsInvalid(err), apierrors.IsResourceExpired(err):
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	case apierrors.IsAlreadyExists(err):
		return fmt.Errorf("%w: %w", ErrAlreadyExists, err)
	case apierrors.IsNotFound(err):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case apierrors.IsConflict(err):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	default:
		return err
	}
}
