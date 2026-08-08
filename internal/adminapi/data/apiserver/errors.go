package apiserver

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

func resourceError(operation, kind, id string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		err = fmt.Errorf("%w: %v", biz.ErrResourceNotFound, err)
	case apierrors.IsConflict(err):
		err = fmt.Errorf("%w: %v", biz.ErrResourceVersionConflict, err)
	}
	if id == "" {
		return fmt.Errorf("%s %s: %w", operation, kind, err)
	}
	return fmt.Errorf("%s %s %q: %w", operation, kind, id, err)
}
