package aibackend

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lgc202/ingate/internal/adminapi/repository"
)

func TestTranslateRepositoryErrorPreservesCause(t *testing.T) {
	for _, tt := range []struct {
		name  string
		cause error
		want  error
	}{
		{name: "invalid", cause: repository.ErrInvalid, want: ErrInvalidArgument},
		{name: "identity conflict", cause: repository.ErrAlreadyExists, want: ErrAlreadyExists},
		{name: "not found", cause: repository.ErrNotFound, want: ErrNotFound},
		{name: "version conflict", cause: repository.ErrConflict, want: ErrConflict},
		{name: "canceled", cause: context.Canceled, want: context.Canceled},
		{name: "deadline", cause: context.DeadlineExceeded, want: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Errorf("storage context: %w", tt.cause)
			got := translateRepositoryError(input)
			if !errors.Is(got, tt.want) || !errors.Is(got, input) || !errors.Is(got, tt.cause) {
				t.Errorf("translateRepositoryError(%s) = %v, want %v with full cause chain", tt.name, got, tt.want)
			}
		})
	}
	if got := translateRepositoryError(nil); got != nil {
		t.Errorf("translateRepositoryError(nil) = %v, want nil", got)
	}
}
