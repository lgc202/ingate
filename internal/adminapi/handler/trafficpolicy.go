package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type TrafficPolicyHandler struct {
	store *store.APIServerStore
}

func NewTrafficPolicyHandler(store *store.APIServerStore) *TrafficPolicyHandler {
	return &TrafficPolicyHandler{store: store}
}

func (h *TrafficPolicyHandler) Create(c *gin.Context) {
	var req dto.CreateTrafficPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	created, err := h.store.CreateTrafficPolicy(c.Request.Context(), convert.TrafficPolicyFromCreateRequest(req))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, convert.TrafficPolicyToResponse(created))
}

func (h *TrafficPolicyHandler) Update(c *gin.Context) {
	var req dto.UpdateTrafficPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	current, err := h.store.GetTrafficPolicy(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	updated := convert.TrafficPolicyFromUpdateRequest(c.Param("name"), req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := h.store.UpdateTrafficPolicy(c.Request.Context(), updated)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.TrafficPolicyToResponse(result))
}

func (h *TrafficPolicyHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteTrafficPolicy(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TrafficPolicyHandler) Get(c *gin.Context) {
	policy, err := h.store.GetTrafficPolicy(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.TrafficPolicyToResponse(policy))
}

func (h *TrafficPolicyHandler) List(c *gin.Context) {
	list, err := h.store.ListTrafficPolicies(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.TrafficPolicyListToResponse(list))
}
