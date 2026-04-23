package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type GatewayHandler struct {
	store *store.APIServerStore
}

func NewGatewayHandler(store *store.APIServerStore) *GatewayHandler {
	return &GatewayHandler{store: store}
}

func (h *GatewayHandler) Create(c *gin.Context) {
	var req dto.CreateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	created, err := h.store.CreateGateway(c.Request.Context(), convert.GatewayFromCreateRequest(req))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, convert.GatewayToResponse(created))
}

func (h *GatewayHandler) Update(c *gin.Context) {
	var req dto.UpdateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	current, err := h.store.GetGateway(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	updated := convert.GatewayFromUpdateRequest(c.Param("name"), req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := h.store.UpdateGateway(c.Request.Context(), updated)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.GatewayToResponse(result))
}

func (h *GatewayHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteGateway(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *GatewayHandler) Get(c *gin.Context) {
	gateway, err := h.store.GetGateway(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.GatewayToResponse(gateway))
}

func (h *GatewayHandler) List(c *gin.Context) {
	list, err := h.store.ListGateways(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.GatewayListToResponse(list))
}
