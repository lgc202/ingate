package validation

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func TestValidateConfigMap(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]string
		binaryData map[string][]byte
		wantValid  bool
	}{
		{name: "empty", wantValid: true},
		{
			name: "string_and_binary", wantValid: true,
			data:       map[string]string{"config.json": "配置"},
			binaryData: map[string][]byte{"bytes": {0, 255}},
		},
		{name: "invalid_data_key", data: map[string]string{"bad/key": "value"}},
		{name: "invalid_binary_key", binaryData: map[string][]byte{"bad/key": {1}}},
		{
			name:       "duplicate_key",
			data:       map[string]string{"key": "value"},
			binaryData: map[string][]byte{"key": {1}},
		},
		{
			name: "exact_limit", wantValid: true,
			data:       map[string]string{"text": strings.Repeat("x", corev1.MaxSecretSize-1)},
			binaryData: map[string][]byte{"bytes": {0}},
		},
		{
			name:       "combined_over_limit",
			data:       map[string]string{"text": strings.Repeat("x", corev1.MaxSecretSize)},
			binaryData: map[string][]byte{"bytes": {0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap := &coreapi.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime"},
				Data:       tt.data,
				BinaryData: tt.binaryData,
			}
			errs := ValidateConfigMap(configMap)
			if valid := len(errs) == 0; valid != tt.wantValid {
				t.Errorf("ValidateConfigMap(%s) errors = %v, want valid %t", tt.name, errs, tt.wantValid)
			}
		})
	}
}

func TestValidateConfigMapMetadata(t *testing.T) {
	for _, tt := range []struct {
		name     string
		metadata metav1.ObjectMeta
	}{
		{name: "missing_namespace", metadata: metav1.ObjectMeta{Name: "runtime"}},
		{name: "invalid_name", metadata: metav1.ObjectMeta{Namespace: "team-a", Name: "Not.Valid"}},
		{name: "unqualified_finalizer", metadata: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime", Finalizers: []string{"custom"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if errs := ValidateConfigMap(&coreapi.ConfigMap{ObjectMeta: tt.metadata}); len(errs) == 0 {
				t.Errorf("ValidateConfigMap(metadata=%+v) errors = empty, want invalid metadata", tt.metadata)
			}
		})
	}
}

func TestValidateConfigMapUpdate(t *testing.T) {
	previous := &coreapi.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime", ResourceVersion: "1"},
		Immutable:  ptr.To(true),
		Data:       map[string]string{"config.json": "{}"},
		BinaryData: map[string][]byte{"bytes": {0, 255}},
	}
	tests := []struct {
		name      string
		mutate    func(*coreapi.ConfigMap)
		wantValid bool
	}{
		{name: "unchanged", mutate: func(*coreapi.ConfigMap) {}, wantValid: true},
		{name: "metadata_change", mutate: func(c *coreapi.ConfigMap) { c.Labels = map[string]string{"owner": "platform"} }, wantValid: true},
		{name: "data_change", mutate: func(c *coreapi.ConfigMap) { c.Data["config.json"] = "other" }},
		{name: "binary_change", mutate: func(c *coreapi.ConfigMap) { c.BinaryData["bytes"][0] = 1 }},
		{name: "clear_immutable", mutate: func(c *coreapi.ConfigMap) { c.Immutable = nil }},
		{name: "unset_immutable", mutate: func(c *coreapi.ConfigMap) { c.Immutable = ptr.To(false) }},
		{name: "change_namespace", mutate: func(c *coreapi.ConfigMap) { c.Namespace = "team-b" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := previous.DeepCopy()
			tt.mutate(next)
			errs := ValidateConfigMapUpdate(next, previous)
			if valid := len(errs) == 0; valid != tt.wantValid {
				t.Errorf("ValidateConfigMapUpdate(%s) errors = %v, want valid %t", tt.name, errs, tt.wantValid)
			}
		})
	}

	mutable := previous.DeepCopy()
	mutable.Immutable = nil
	next := mutable.DeepCopy()
	next.Data["config.json"] = "updated"
	if errs := ValidateConfigMapUpdate(next, mutable); len(errs) != 0 {
		t.Errorf("ValidateConfigMapUpdate(mutable data) errors = %v, want valid", errs)
	}
}
