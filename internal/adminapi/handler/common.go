package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

const (
	ErrorCodeBadRequest = "BadRequest"
	ErrorCodeNotFound   = "NotFound"
	ErrorCodeConflict   = "Conflict"
	ErrorCodeInternal   = "InternalError"
)

func writeError(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, dto.ErrorResponse{Code: code, Message: message})
}

func writeBindError(c *gin.Context, err error) {
	writeError(c, http.StatusBadRequest, ErrorCodeBadRequest, err.Error())
}

func writeStoreError(c *gin.Context, err error) {
	switch {
	case apierrors.IsNotFound(err):
		writeError(c, http.StatusNotFound, ErrorCodeNotFound, err.Error())
	case apierrors.IsAlreadyExists(err):
		writeError(c, http.StatusConflict, ErrorCodeConflict, err.Error())
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		writeError(c, http.StatusBadRequest, ErrorCodeBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, ErrorCodeInternal, err.Error())
	}
}
