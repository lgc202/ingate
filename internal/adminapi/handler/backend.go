package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

type BackendHandler struct {
	service *biz.BackendService
}

func NewBackendHandler(service *biz.BackendService) *BackendHandler {
	return &BackendHandler{service: service}
}

func (h *BackendHandler) Create(c *gin.Context) {
	var req dto.CreateBackendRequest
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

func (h *BackendHandler) Update(c *gin.Context) {
	var req dto.UpdateBackendRequest
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

func (h *BackendHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BackendHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *BackendHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
