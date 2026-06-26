package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/store"
)

type InventoryHandler struct {
	Store *store.InventoryStore
}

func (h *InventoryHandler) GetInventory(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.Store.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusOK, gin.H{
			"ports":    []any{},
			"services": []any{},
			"packages": []any{},
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ts":       snap.TS,
		"ports":    snap.Ports,
		"services": snap.Services,
		"packages": snap.Packages,
	})
}
