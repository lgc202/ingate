package reservedmetadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

const (
	PluginName                       = "IngateReservedMetadata"
	ReservedMetadataPrefix           = "internal.ingate.io/"
	ErrReservedMetadataPrefixPattern = "metadata keys with prefix %q are reserved for system use"
)

type plugin struct {
	*admission.Handler
}

var _ admission.ValidationInterface = &plugin{}

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(_ io.Reader) (admission.Interface, error) {
		return New(), nil
	})
}

func New() admission.Interface {
	return &plugin{Handler: admission.NewHandler(admission.Create, admission.Update)}
}

func (p *plugin) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	obj, ok := a.GetObject().(metav1.Object)
	if !ok {
		return nil
	}

	if key, found := findReservedMetadataKey(obj.GetLabels()); found {
		return admission.NewForbidden(a, errors.New(newReservedMetadataError(key)))
	}
	if key, found := findReservedMetadataKey(obj.GetAnnotations()); found {
		return admission.NewForbidden(a, errors.New(newReservedMetadataError(key)))
	}
	return nil
}

func findReservedMetadataKey(values map[string]string) (string, bool) {
	for key := range values {
		if strings.HasPrefix(key, ReservedMetadataPrefix) {
			return key, true
		}
	}
	return "", false
}

func newReservedMetadataError(key string) string {
	return fmt.Sprintf(ErrReservedMetadataPrefixPattern+": %s", ReservedMetadataPrefix, key)
}
