package shared

import (
	"fmt"
	"strings"
)

// ObjectKey is a compact object identifier that supports both cluster-scoped
// objects ("name") and namespaced objects ("namespace/name").
type ObjectKey struct {
	Namespace string
	Name      string
}

func NewObjectKey(namespace, name string) ObjectKey {
	return ObjectKey{
		Namespace: strings.TrimSpace(namespace),
		Name:      strings.TrimSpace(name),
	}
}

func (k ObjectKey) String() string {
	switch {
	case k.Namespace == "":
		return k.Name
	case k.Name == "":
		return k.Namespace + "/"
	default:
		return k.Namespace + "/" + k.Name
	}
}

func ParseObjectKey(value string) (ObjectKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ObjectKey{}, fmt.Errorf("object key must not be empty")
	}

	parts := strings.Split(value, "/")
	switch len(parts) {
	case 1:
		if parts[0] == "" {
			return ObjectKey{}, fmt.Errorf("object key name must not be empty")
		}
		return NewObjectKey("", parts[0]), nil
	case 2:
		if parts[0] == "" {
			return ObjectKey{}, fmt.Errorf("object key namespace must not be empty")
		}
		if parts[1] == "" {
			return ObjectKey{}, fmt.Errorf("object key name must not be empty")
		}
		return NewObjectKey(parts[0], parts[1]), nil
	default:
		return ObjectKey{}, fmt.Errorf("invalid object key %q", value)
	}
}
