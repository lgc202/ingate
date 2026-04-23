package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type BackendHandler struct {
	store *store.APIServerStore
}

func NewBackendHandler(store *store.APIServerStore) *BackendHandler {
	return &BackendHandler{store: store}
}

func (h *BackendHandler) Create(c *gin.Context) {
	var req dto.CreateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	created, err := h.store.CreateBackend(c.Request.Context(), convert.BackendFromCreateRequest(req))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, convert.BackendToResponse(created))
}

func (h *BackendHandler) Update(c *gin.Context) {
	var req dto.UpdateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	current, err := h.store.GetBackend(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	updated := convert.BackendFromUpdateRequest(c.Param("name"), req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := h.store.UpdateBackend(c.Request.Context(), updated)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.BackendToResponse(result))
}

func (h *BackendHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteBackend(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BackendHandler) Get(c *gin.Context) {
	backend, err := h.store.GetBackend(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.BackendToResponse(backend))
}

func (h *BackendHandler) List(c *gin.Context) {
	list, err := h.store.ListBackends(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.BackendListToResponse(list))
}
