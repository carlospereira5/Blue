// Package api implementa los handlers HTTP de Blue.
package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthHandler retorna el estado real de la DB.
// GET /health → {"status":"ok"} o 503 {"status":"error","message":"..."}
func healthHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
