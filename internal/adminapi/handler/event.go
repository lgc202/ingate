package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

type EventHandler struct {
	service *biz.EventService
}

func NewEventHandler(service *biz.EventService) *EventHandler {
	return &EventHandler{service: service}
}

func (h *EventHandler) List(c *gin.Context) {
	resp, err := h.service.ListEvents(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
