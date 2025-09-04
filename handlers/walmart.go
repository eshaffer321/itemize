package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"monarchmoney-sync-backend/models"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates requests using the X-Extension-Key header.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		extensionKey := c.GetHeader("X-Extension-Key")
		expectedKey := os.Getenv("EXTENSION_SECRET_KEY")

		// For testing, allow "test-secret" when env var is not set
		if expectedKey == "" {
			expectedKey = "test-secret"
		}

		if extensionKey == "" || extensionKey != expectedKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Unauthorized: Missing or invalid extension key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ReceiveOrders handles incoming Walmart order data from the Chrome extension.
func ReceiveOrders(c *gin.Context) {
	// Get Sentry hub from context if available
	hub := sentrygin.GetHubFromContext(c)

	var order models.Order

	// Parse JSON request body
	if err := c.ShouldBindJSON(&order); err != nil {
		// Capture validation errors to Sentry
		if hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelWarning)
				scope.SetContext("order", map[string]interface{}{
					"error": err.Error(),
					"body":  c.Request.Body,
				})
				hub.CaptureMessage("Invalid order JSON received")
			})
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Invalid JSON or validation error: %v", err),
		})
		return
	}

	// Validate items if present
	if order.Items != nil {
		for _, item := range order.Items {
			if item.Price < 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Invalid item price: must be non-negative",
				})
				return
			}
			if item.Quantity <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Invalid item quantity: must be positive",
				})
				return
			}
		}
	}

	// Generate processing ID
	processingID := fmt.Sprintf("proc_%s_%d", order.OrderNumber, time.Now().Unix())

	// Calculate item count
	itemCount := 0
	if order.Items != nil {
		itemCount = len(order.Items)
	}

	// Log the received order with additional fields
	logMsg := fmt.Sprintf("Received Walmart order: %s", order.OrderNumber)
	if order.OrderTotal != nil {
		logMsg += fmt.Sprintf(", Total: $%.2f", *order.OrderTotal)
	}
	logMsg += fmt.Sprintf(", Items: %d", itemCount)
	if order.Tax != nil {
		logMsg += fmt.Sprintf(", Tax: $%.2f", *order.Tax)
	}
	if order.DeliveryCharges != nil {
		logMsg += fmt.Sprintf(", Delivery: $%.2f", *order.DeliveryCharges)
	}
	if order.Tip != nil {
		logMsg += fmt.Sprintf(", Tip: $%.2f", *order.Tip)
	}
	log.Println(logMsg)

	// Track successful order in Sentry
	if hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelInfo)
			contextData := map[string]interface{}{
				"order_number":  order.OrderNumber,
				"items_count":   itemCount,
				"processing_id": processingID,
			}
			if order.OrderTotal != nil {
				contextData["total"] = *order.OrderTotal
			}
			if order.Tax != nil {
				contextData["tax"] = *order.Tax
			}
			if order.DeliveryCharges != nil {
				contextData["delivery_charges"] = *order.DeliveryCharges
			}
			if order.Tip != nil {
				contextData["tip"] = *order.Tip
			}
			scope.SetContext("order", contextData)
			scope.SetTag("order.source", "walmart")
			hub.CaptureMessage("Order received successfully")
		})
	}

	// Update sync tracker
	updateSyncTracker(&order)

	// TODO: Process order with Monarch Money SDK
	// For now, just acknowledge receipt

	response := models.OrderResponse{
		Status:       "success",
		Message:      "Order received successfully",
		OrderID:      order.OrderNumber,
		ProcessingID: processingID,
		ItemCount:    itemCount,
		TotalAmount:  order.OrderTotal,
		Timestamp:    time.Now(),
	}

	c.JSON(http.StatusOK, response)
}
