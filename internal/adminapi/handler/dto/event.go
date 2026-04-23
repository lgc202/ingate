package dto

type EventListResponse struct {
	Items []Event `json:"items"`
}

type Event struct {
	ID       string `json:"id"`
	Level    string `json:"level"`
	Message  string `json:"message"`
	Resource string `json:"resource"`
	Time     string `json:"time"`
}
