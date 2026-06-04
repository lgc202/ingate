package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestEventHandlerSkipsUpdateWhenResourceVersionUnchanged(t *testing.T) {
	enqueued := 0
	handler := (&Controller{}).eventHandler(func(any) {
		enqueued++
	})

	handler.UpdateFunc(
		&resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", ResourceVersion: "1"}},
		&resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", ResourceVersion: "1"}},
	)

	if enqueued != 0 {
		t.Fatalf("UpdateFunc enqueued %d objects, want 0", enqueued)
	}
}

func TestEventHandlerEnqueuesUpdateWhenResourceVersionChanged(t *testing.T) {
	enqueued := 0
	handler := (&Controller{}).eventHandler(func(any) {
		enqueued++
	})

	handler.UpdateFunc(
		&resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", ResourceVersion: "1"}},
		&resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", ResourceVersion: "2"}},
	)

	if enqueued != 1 {
		t.Fatalf("UpdateFunc enqueued %d objects, want 1", enqueued)
	}
}
