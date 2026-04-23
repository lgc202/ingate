package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

type CertificateHandler struct {
	service *biz.CertificateService
}

func NewCertificateHandler(service *biz.CertificateService) *CertificateHandler {
	return &CertificateHandler{service: service}
}

func (h *CertificateHandler) Create(c *gin.Context) {
	var req dto.CreateCertificateRequest
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

func (h *CertificateHandler) Update(c *gin.Context) {
	var req dto.UpdateCertificateRequest
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

func (h *CertificateHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CertificateHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *CertificateHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *CertificateHandler) ListSecrets(c *gin.Context) {
	resp, err := h.service.ListSecretOptions(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
