package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

type RouteHandler struct {
	service *biz.RouteService
}

func NewRouteHandler(service *biz.RouteService) *RouteHandler {
	return &RouteHandler{service: service}
}

func (h *RouteHandler) Create(c *gin.Context) {
	var req dto.CreateRouteRequest
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

func (h *RouteHandler) Update(c *gin.Context) {
	var req dto.UpdateRouteRequest
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

func (h *RouteHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RouteHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *RouteHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
