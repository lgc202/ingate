package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

type GatewayHandler struct {
	service *biz.GatewayService
}

func NewGatewayHandler(service *biz.GatewayService) *GatewayHandler {
	return &GatewayHandler{service: service}
}

func (h *GatewayHandler) Create(c *gin.Context) {
	var req dto.CreateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	resp, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *GatewayHandler) Update(c *gin.Context) {
	var req dto.UpdateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	resp, err := h.service.Update(c.Request.Context(), c.Param("name"), req)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *GatewayHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *GatewayHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *GatewayHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
