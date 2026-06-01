package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
)

// WriteResult 输出统一 JSON 结果，并把 apiserver 常见错误映射为 HTTP 状态码
func WriteResult(ctx *gin.Context, value any, err error) {
	if err == nil {
		ctx.JSON(http.StatusOK, value)
		return
	}
	if apierrors.IsNotFound(err) {
		WriteError(ctx, http.StatusNotFound, "resource not found")
		return
	}
	if apierrors.IsBadRequest(err) {
		WriteError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if apierrors.IsAlreadyExists(err) {
		WriteError(ctx, http.StatusConflict, err.Error())
		return
	}
	if apierrors.IsConflict(err) {
		WriteError(ctx, http.StatusConflict, err.Error())
		return
	}
	WriteError(ctx, http.StatusInternalServerError, err.Error())
}

// WriteError 输出统一 JSON 错误响应
func WriteError(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, gin.H{"error": message, "requestID": ctx.GetString(requestid.Header)})
}
