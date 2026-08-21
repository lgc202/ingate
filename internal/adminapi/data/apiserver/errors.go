package apiserver

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

func listOptions(page biz.PageRequest) metav1.ListOptions {
	return metav1.ListOptions{Limit: page.Limit, Continue: page.Cursor}
}

func listError(kind string, err error) error {
	if apierrors.IsBadRequest(err) || apierrors.IsResourceExpired(err) {
		return fmt.Errorf("%w: %w", biz.ErrInvalidCursor, err)
	}
	return resourceError("list", kind, "", err)
}

func resourceError(operation, kind, name string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		err = fmt.Errorf("%w: %w", biz.ErrResourceNotFound, err)
	case apierrors.IsConflict(err):
		err = fmt.Errorf("%w: %w", biz.ErrResourceVersionConflict, err)
	}
	if name == "" {
		return fmt.Errorf("%s %s: %w", operation, kind, err)
	}
	return fmt.Errorf("%s %s %q: %w", operation, kind, name, err)
}
