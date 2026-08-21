package server

import (
	"encoding/json"
	"net/http"
)

func writeJSON(response http.ResponseWriter, status int, value any) error {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	return json.NewEncoder(response).Encode(value)
}
