// Package handlers contains HTTP request handlers for the Walmart-Monarch sync backend.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns a simple health status response.
// This endpoint is used by load balancers and monitoring systems.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
