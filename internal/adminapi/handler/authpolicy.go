package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type AuthPolicyHandler struct {
	store *store.APIServerStore
}

func NewAuthPolicyHandler(store *store.APIServerStore) *AuthPolicyHandler {
	return &AuthPolicyHandler{store: store}
}

func (h *AuthPolicyHandler) Create(c *gin.Context) {
	var req dto.CreateAuthPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	created, err := h.store.CreateAuthPolicy(c.Request.Context(), convert.AuthPolicyFromCreateRequest(req))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, convert.AuthPolicyToResponse(created))
}

func (h *AuthPolicyHandler) Update(c *gin.Context) {
	var req dto.UpdateAuthPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}

	current, err := h.store.GetAuthPolicy(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	updated := convert.AuthPolicyFromUpdateRequest(c.Param("name"), req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := h.store.UpdateAuthPolicy(c.Request.Context(), updated)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.AuthPolicyToResponse(result))
}

func (h *AuthPolicyHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteAuthPolicy(c.Request.Context(), c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthPolicyHandler) Get(c *gin.Context) {
	policy, err := h.store.GetAuthPolicy(c.Request.Context(), c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.AuthPolicyToResponse(policy))
}

func (h *AuthPolicyHandler) List(c *gin.Context) {
	list, err := h.store.ListAuthPolicies(c.Request.Context())
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, convert.AuthPolicyListToResponse(list))
}
