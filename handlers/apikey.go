package handlers

import (
	"net/http"

	"jimeng-service/models"
	"jimeng-service/services"

	"github.com/gin-gonic/gin"
)

type APIKeyHandler struct {
	keyPool *services.KeyPool
}

func NewAPIKeyHandler(keyPool *services.KeyPool) *APIKeyHandler {
	return &APIKeyHandler{keyPool: keyPool}
}

type AddKeyRequest struct {
	AK     string `json:"ak" binding:"required"`
	SK     string `json:"sk" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Weight int    `json:"weight"`
}

type UpdateKeyRequest struct {
	Name   string                 `json:"name"`
	Weight int                    `json:"weight"`
	Quotas map[string]models.Quota `json:"quotas"`
}

func (h *APIKeyHandler) GetKeys(c *gin.Context) {
	keys := h.keyPool.GetKeys()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": keys,
	})
}

func (h *APIKeyHandler) AddKey(c *gin.Context) {
	var req AddKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	if req.Weight <= 0 {
		req.Weight = 10
	}

	newKey, err := h.keyPool.AddKey(req.AK, req.SK, req.Name, req.Weight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to add key: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": newKey,
	})
}

func (h *APIKeyHandler) UpdateKey(c *gin.Context) {
	id := c.Param("id")

	var req UpdateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.keyPool.UpdateKey(id, req.Name, req.Weight, req.Quotas); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	key, _ := h.keyPool.GetKeyByID(id)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": key,
	})
}

func (h *APIKeyHandler) DeleteKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.keyPool.DeleteKey(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "deleted successfully",
	})
}

type ImportKeysRequest struct {
	Keys []*models.APIKey `json:"keys" binding:"required"`
}

func (h *APIKeyHandler) ImportKeys(c *gin.Context) {
	var req ImportKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.keyPool.ImportKeys(req.Keys); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to import keys: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "imported successfully",
		"count":   len(req.Keys),
	})
}

func (h *APIKeyHandler) GetStatus(c *gin.Context) {
	status := h.keyPool.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": status,
	})
}

func (h *APIKeyHandler) ResetKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.keyPool.ResetKey(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	key, _ := h.keyPool.GetKeyByID(id)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": key,
	})
}
