package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

type OverviewHandler struct {
	service *biz.OverviewService
}

func NewOverviewHandler(service *biz.OverviewService) *OverviewHandler {
	return &OverviewHandler{service: service}
}

func (h *OverviewHandler) GetOverview(c *gin.Context) {
	resp, err := h.service.GetOverview(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
