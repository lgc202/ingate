package gatewaycompiler

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestShouldRequeueOnlyRetryableErrors(t *testing.T) {
	resource := schema.GroupResource{Group: "gateway.ingate.io", Resource: "gateways"}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not found", err: apierrors.NewNotFound(resource, "public-edge"), want: false},
		{name: "conflict", err: apierrors.NewConflict(resource, "public-edge", errors.New("conflict")), want: true},
		{name: "network timeout", err: timeoutError{}, want: true},
		{name: "generic", err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRequeue(tt.err); got != tt.want {
				t.Fatalf("shouldRequeue(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ interface {
	error
	Timeout() bool
	Temporary() bool
} = timeoutError{}
