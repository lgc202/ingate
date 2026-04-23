package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type RouteHandler struct {
	store *store.APIServerStore
}

func NewRouteHandler(store *store.APIServerStore) *RouteHandler {
	return &RouteHandler{store: store}
}

func (h *RouteHandler) Create(c *gin.Context) {
	var req dto.CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	created, err := h.store.CreateRoute(c.Request.Context(), convert.RouteFromCreateRequest(req))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, convert.RouteToResponse(created))
}

func (h *RouteHandler) Update(c *gin.Context) {
	var req dto.UpdateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	current, err := h.store.GetRoute(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	updated := convert.RouteFromUpdateRequest(c.Param("name"), req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := h.store.UpdateRoute(c.Request.Context(), updated)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.RouteToResponse(result))
}

func (h *RouteHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteRoute(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RouteHandler) Get(c *gin.Context) {
	route, err := h.store.GetRoute(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.RouteToResponse(route))
}

func (h *RouteHandler) List(c *gin.Context) {
	list, err := h.store.ListRoutes(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.RouteListToResponse(list))
}
