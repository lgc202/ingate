package repository

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestTranslateResourceError(t *testing.T) {
	resource := schema.GroupResource{Group: "gateway.ingate.io", Resource: "httproutes"}
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "invalid", err: apierrors.NewBadRequest("invalid page token"), want: ErrInvalid},
		{name: "already exists", err: apierrors.NewAlreadyExists(resource, "public"), want: ErrAlreadyExists},
		{name: "not found", err: apierrors.NewNotFound(resource, "public"), want: ErrNotFound},
		{name: "conflict", err: apierrors.NewConflict(resource, "public", errors.New("stale version")), want: ErrConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := translateResourceError(tt.err); !errors.Is(err, tt.want) {
				t.Errorf("translateResourceError(%v) = %v, want %v", tt.err, err, tt.want)
			}
		})
	}
}
