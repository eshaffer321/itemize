package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrder_Unmarshal_AllFields(t *testing.T) {
	// Test that Order can unmarshal all fields including new optional ones
	jsonData := `{
		"orderNumber": "123456",
		"orderDate": "2024-01-15",
		"orderTotal": 150.50,
		"tax": 12.50,
		"deliveryCharges": 5.99,
		"tip": 10.00,
		"items": [
			{
				"name": "Great Value Milk",
				"price": 3.99,
				"quantity": 2,
				"productUrl": "https://walmart.com/product/123",
				"category": "Groceries"
			}
		]
	}`

	var order Order
	err := json.Unmarshal([]byte(jsonData), &order)

	assert.NoError(t, err)
	assert.Equal(t, "123456", order.OrderNumber)
	assert.Equal(t, "2024-01-15", order.OrderDate)
	assert.Equal(t, 150.50, *order.OrderTotal)
	assert.Equal(t, 12.50, *order.Tax)
	assert.Equal(t, 5.99, *order.DeliveryCharges)
	assert.Equal(t, 10.00, *order.Tip)
	assert.Len(t, order.Items, 1)
	assert.Equal(t, "Great Value Milk", order.Items[0].Name)
	assert.Equal(t, "https://walmart.com/product/123", order.Items[0].ProductURL)
	assert.Equal(t, "Groceries", order.Items[0].Category)
}

func TestOrder_Unmarshal_MinimalFields(t *testing.T) {
	// Test that Order works with only required fields
	jsonData := `{
		"orderNumber": "123456",
		"orderDate": "2024-01-15"
	}`

	var order Order
	err := json.Unmarshal([]byte(jsonData), &order)

	assert.NoError(t, err)
	assert.Equal(t, "123456", order.OrderNumber)
	assert.Equal(t, "2024-01-15", order.OrderDate)
	assert.Nil(t, order.OrderTotal)
	assert.Nil(t, order.Tax)
	assert.Nil(t, order.DeliveryCharges)
	assert.Nil(t, order.Tip)
	assert.Nil(t, order.Items)
}

func TestOrder_Unmarshal_WithoutOrderTotal(t *testing.T) {
	// Test that Order works without orderTotal
	jsonData := `{
		"orderNumber": "123456",
		"orderDate": "2024-01-15",
		"items": [
			{
				"name": "Test Item",
				"price": 9.99,
				"quantity": 1
			}
		]
	}`

	var order Order
	err := json.Unmarshal([]byte(jsonData), &order)

	assert.NoError(t, err)
	assert.Nil(t, order.OrderTotal)
	assert.Len(t, order.Items, 1)
}

func TestOrder_Unmarshal_WithoutItems(t *testing.T) {
	// Test that Order works without items
	jsonData := `{
		"orderNumber": "123456",
		"orderDate": "2024-01-15",
		"orderTotal": 50.00
	}`

	var order Order
	err := json.Unmarshal([]byte(jsonData), &order)

	assert.NoError(t, err)
	assert.Equal(t, 50.00, *order.OrderTotal)
	assert.Nil(t, order.Items)
}

func TestOrderItem_WithProductURL(t *testing.T) {
	// Test OrderItem with product URL
	jsonData := `{
		"name": "Test Product",
		"price": 19.99,
		"quantity": 2,
		"productUrl": "https://walmart.com/product/456"
	}`

	var item OrderItem
	err := json.Unmarshal([]byte(jsonData), &item)

	assert.NoError(t, err)
	assert.Equal(t, "Test Product", item.Name)
	assert.Equal(t, 19.99, item.Price)
	assert.Equal(t, 2, item.Quantity)
	assert.Equal(t, "https://walmart.com/product/456", item.ProductURL)
}

func TestBatchOrdersRequest(t *testing.T) {
	// Test BatchOrdersRequest structure
	jsonData := `{
		"orders": [
			{
				"orderNumber": "123",
				"orderDate": "2024-01-15"
			},
			{
				"orderNumber": "456",
				"orderDate": "2024-01-16",
				"orderTotal": 75.50
			}
		]
	}`

	var batchRequest BatchOrdersRequest
	err := json.Unmarshal([]byte(jsonData), &batchRequest)

	assert.NoError(t, err)
	assert.Len(t, batchRequest.Orders, 2)
	assert.Equal(t, "123", batchRequest.Orders[0].OrderNumber)
	assert.Equal(t, "456", batchRequest.Orders[1].OrderNumber)
	assert.Equal(t, 75.50, *batchRequest.Orders[1].OrderTotal)
}

