package server

import (
	"encoding/json"
	"net/http"
)

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Data    any    `json:"data"`
}

func writeResponse(response http.ResponseWriter, statusCode int, message string, data any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(statusCode)
	// 写出响应头后，编码失败只能结束当前连接，调用方无法再发送另一份 HTTP 响应。
	_ = json.NewEncoder(response).Encode(apiResponse{
		Code:    statusCode,
		Message: message,
		Data:    data,
	})
}
