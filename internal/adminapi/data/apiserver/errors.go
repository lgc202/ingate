package apiserver

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

func pageOptions(page biz.PageRequest) metav1.ListOptions {
	return metav1.ListOptions{Limit: page.Limit, Continue: page.Cursor}
}

func pageError(kind string, err error) error {
	if apierrors.IsBadRequest(err) || apierrors.IsResourceExpired(err) {
		return fmt.Errorf("%w: %v", biz.ErrInvalidCursor, err)
	}
	return resourceError("list", kind, "", err)
}

func resourceError(operation, kind, id string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		err = fmt.Errorf("%w: %v", biz.ErrResourceNotFound, err)
	case apierrors.IsAlreadyExists(err):
		err = fmt.Errorf("%w: %v", biz.ErrDisplayNameConflict, err)
	case apierrors.IsConflict(err):
		err = fmt.Errorf("%w: %v", biz.ErrResourceVersionConflict, err)
	}
	if id == "" {
		return fmt.Errorf("%s %s: %w", operation, kind, err)
	}
	return fmt.Errorf("%s %s %q: %w", operation, kind, id, err)
}
