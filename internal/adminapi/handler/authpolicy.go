package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

type AuthPolicyHandler struct {
	service *biz.AuthPolicyService
}

func NewAuthPolicyHandler(service *biz.AuthPolicyService) *AuthPolicyHandler {
	return &AuthPolicyHandler{service: service}
}

func (h *AuthPolicyHandler) Create(c *gin.Context) {
	var req dto.CreateAuthPolicyRequest
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

func (h *AuthPolicyHandler) Update(c *gin.Context) {
	var req dto.UpdateAuthPolicyRequest
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

func (h *AuthPolicyHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthPolicyHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthPolicyHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
