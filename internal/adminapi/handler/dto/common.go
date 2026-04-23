package dto

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ObjectMeta struct {
	Name              string `json:"name"`
	ResourceVersion   string `json:"resourceVersion,omitempty"`
	Generation        int64  `json:"generation,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

func NewObjectMeta(meta metav1.ObjectMeta) ObjectMeta {
	return ObjectMeta{
		Name:              meta.Name,
		ResourceVersion:   meta.ResourceVersion,
		Generation:        meta.Generation,
		CreationTimestamp: meta.CreationTimestamp.String(),
	}
}

func NewConditions(conditions []metav1.Condition) []Condition {
	items := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		items = append(items, Condition{
			Type:               condition.Type,
			Status:             string(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			ObservedGeneration: condition.ObservedGeneration,
		})
	}
	return items
}
