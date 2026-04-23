package dto

type ResourceRef struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

type TopologyResponse struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type TopologyEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}
