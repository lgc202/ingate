package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const healthStatusOK = "ok"

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Healthz(c *gin.Context) {
	c.String(http.StatusOK, healthStatusOK)
}

func (h *HealthHandler) Readyz(c *gin.Context) {
	c.String(http.StatusOK, healthStatusOK)
}
