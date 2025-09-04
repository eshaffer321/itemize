package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"monarchmoney-sync-backend/models"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// ReceiveBatchOrders handles multiple Walmart orders in a single request.
func ReceiveBatchOrders(c *gin.Context) {
	// Get Sentry hub from context if available
	hub := sentrygin.GetHubFromContext(c)

	var batchRequest models.BatchOrdersRequest

	// Parse JSON request body
	if err := c.ShouldBindJSON(&batchRequest); err != nil {
		// Capture validation errors to Sentry
		if hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelWarning)
				scope.SetContext("batch", map[string]interface{}{
					"error": err.Error(),
					"body":  c.Request.Body,
				})
				hub.CaptureMessage("Invalid batch JSON received")
			})
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Invalid JSON or validation error: %v", err),
		})
		return
	}

	// Validate batch has orders
	if len(batchRequest.Orders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No orders provided in batch request",
		})
		return
	}

	// Process each order
	results := make([]models.BatchOrderResult, 0, len(batchRequest.Orders))
	processedCount := 0
	failedCount := 0

	for _, order := range batchRequest.Orders {
		result := models.BatchOrderResult{
			OrderNumber: order.OrderNumber,
		}

		// Validate individual order
		if err := validateOrder(order); err != nil {
			result.Success = false
			result.Error = err.Error()
			failedCount++
		} else {
			// Process the order (same logic as single order endpoint)
			processingID := fmt.Sprintf("proc_%s_%d", order.OrderNumber, time.Now().Unix())

			// Log the order
			logBatchOrder(order)

			// Track in Sentry
			if hub != nil {
				trackBatchOrderInSentry(hub, order, processingID)
			}

			// Update sync tracker
			updateSyncTracker(&order)

			result.Success = true
			result.ProcessingID = processingID
			processedCount++
		}

		results = append(results, result)
	}

	// Log batch summary
	log.Printf("Batch processed: %d successful, %d failed out of %d total orders\n",
		processedCount, failedCount, len(batchRequest.Orders))

	// Determine overall success
	success := processedCount > 0 || failedCount == 0

	response := models.BatchOrdersResponse{
		Success:        success,
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
		Results:        results,
		Timestamp:      time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// validateOrder validates a single order in the batch
func validateOrder(order models.Order) error {
	// Check required fields
	if order.OrderNumber == "" {
		return fmt.Errorf("missing order number")
	}
	if order.OrderDate == "" {
		return fmt.Errorf("missing order date")
	}

	// Validate items if present
	if order.Items != nil {
		for i, item := range order.Items {
			if item.Price < 0 {
				return fmt.Errorf("invalid price for item %d: must be non-negative", i+1)
			}
			if item.Quantity <= 0 {
				return fmt.Errorf("invalid quantity for item %d: must be positive", i+1)
			}
		}
	}

	return nil
}

// logBatchOrder logs a single order from a batch
func logBatchOrder(order models.Order) {
	itemCount := 0
	if order.Items != nil {
		itemCount = len(order.Items)
	}

	logMsg := fmt.Sprintf("Batch order: %s", order.OrderNumber)
	if order.OrderTotal != nil {
		logMsg += fmt.Sprintf(", Total: $%.2f", *order.OrderTotal)
	}
	logMsg += fmt.Sprintf(", Items: %d", itemCount)
	log.Println(logMsg)
}

// trackBatchOrderInSentry tracks a batch order in Sentry
func trackBatchOrderInSentry(hub *sentry.Hub, order models.Order, processingID string) {
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelInfo)

		itemCount := 0
		if order.Items != nil {
			itemCount = len(order.Items)
		}

		contextData := map[string]interface{}{
			"order_number":  order.OrderNumber,
			"items_count":   itemCount,
			"processing_id": processingID,
			"batch":         true,
		}

		if order.OrderTotal != nil {
			contextData["total"] = *order.OrderTotal
		}

		scope.SetContext("order", contextData)
		scope.SetTag("order.source", "walmart.batch")
		hub.CaptureMessage("Batch order processed successfully")
	})
}

