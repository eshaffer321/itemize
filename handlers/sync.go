package handlers

import (
	"net/http"
	"sync"
	"time"

	"monarchmoney-sync-backend/models"

	"github.com/gin-gonic/gin"
)

// SyncTracker tracks synchronization statistics
type SyncTracker struct {
	mu                   sync.RWMutex
	LastSyncTimestamp    *time.Time
	OrdersProcessedToday int
	OrdersProcessedTotal int
	PendingErrors        []string
	todayDate            string
}

var syncTracker = &SyncTracker{
	PendingErrors: []string{},
}

// GetSyncStatus returns the current sync status
func GetSyncStatus(c *gin.Context) {
	syncTracker.mu.RLock()
	defer syncTracker.mu.RUnlock()

	// Reset daily counter if it's a new day
	today := time.Now().Format("2006-01-02")
	if syncTracker.todayDate != today {
		syncTracker.mu.RUnlock()
		syncTracker.mu.Lock()
		syncTracker.OrdersProcessedToday = 0
		syncTracker.todayDate = today
		syncTracker.mu.Unlock()
		syncTracker.mu.RLock()
	}

	status := "operational"
	if len(syncTracker.PendingErrors) > 0 {
		status = "degraded"
	}

	response := models.SyncStatusResponse{
		LastSyncTimestamp:    syncTracker.LastSyncTimestamp,
		OrdersProcessedToday: syncTracker.OrdersProcessedToday,
		OrdersProcessedTotal: syncTracker.OrdersProcessedTotal,
		PendingErrors:        syncTracker.PendingErrors,
		Status:               status,
	}

	c.JSON(http.StatusOK, response)
}

// updateSyncTracker updates the sync tracker with a processed order
func updateSyncTracker(_ *models.Order) {
	syncTracker.mu.Lock()
	defer syncTracker.mu.Unlock()

	now := time.Now()
	syncTracker.LastSyncTimestamp = &now

	// Update daily counter
	today := now.Format("2006-01-02")
	if syncTracker.todayDate != today {
		syncTracker.OrdersProcessedToday = 1
		syncTracker.todayDate = today
	} else {
		syncTracker.OrdersProcessedToday++
	}

	syncTracker.OrdersProcessedTotal++
}


