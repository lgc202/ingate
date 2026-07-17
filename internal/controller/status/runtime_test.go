package status

import (
	"fmt"
	"sync"
	"testing"

	"github.com/lgc202/ingate/internal/envoy/config"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestRuntimeCopiesDiagnostics(t *testing.T) {
	runtime := NewRuntime()
	diagnostics := []config.Diagnostic{{
		Severity: config.SeverityError,
		Kind:     gatewayv1.KindGateway,
		ID:       "gateway-a",
		Reason:   config.ReasonInvalidSpec,
		Message:  "original",
	}}

	runtime.UpdateDiagnostics(diagnostics)
	diagnostics[0].Message = "changed by caller"

	state := runtime.Snapshot()
	if !state.Reconciled {
		t.Error("Snapshot().Reconciled = false, want true")
	}
	if got, want := state.Diagnostics[0].Message, "original"; got != want {
		t.Errorf("Snapshot().Diagnostics[0].Message = %q, want %q", got, want)
	}

	state.Diagnostics[0].Message = "changed snapshot"
	if got, want := runtime.Snapshot().Diagnostics[0].Message, "original"; got != want {
		t.Errorf("second Snapshot().Diagnostics[0].Message = %q, want %q", got, want)
	}
}

func TestRuntimeConcurrentAccess(t *testing.T) {
	runtime := NewRuntime()

	var waitGroup sync.WaitGroup
	for worker := range 8 {
		waitGroup.Add(2)
		go func() {
			defer waitGroup.Done()
			for iteration := range 100 {
				runtime.UpdateDiagnostics([]config.Diagnostic{{
					Severity: config.SeverityWarning,
					ID:       fmt.Sprintf("%d-%d", worker, iteration),
				}})
			}
		}()
		go func() {
			defer waitGroup.Done()
			for range 100 {
				_ = runtime.Snapshot()
			}
		}()
	}
	waitGroup.Wait()
}
