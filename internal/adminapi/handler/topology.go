package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

type TopologyHandler struct {
	service *biz.TopologyService
}

func NewTopologyHandler(service *biz.TopologyService) *TopologyHandler {
	return &TopologyHandler{service: service}
}

func (h *TopologyHandler) GetTopology(c *gin.Context) {
	resp, err := h.service.GetTopology(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
