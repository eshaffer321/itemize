package categorizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Item represents a Walmart item to be categorized
type Item struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity,omitempty"`
}

// Category represents a Monarch Money category
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ItemCategorization represents the categorization result for a single item
type ItemCategorization struct {
	ItemName     string  `json:"item_name"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Confidence   float64 `json:"confidence"`
}

// CategorizationResult contains all categorization results
type CategorizationResult struct {
	Categorizations []ItemCategorization `json:"categorizations"`
}

// OpenAI API types
type ChatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

// OpenAIClient interface for OpenAI API calls
type OpenAIClient interface {
	CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// Cache interface for category mappings
type Cache interface {
	Get(key string) (string, bool)
	Set(key string, value string)
}

// Categorizer handles item categorization using OpenAI
type Categorizer struct {
	client OpenAIClient
	cache  Cache
}

// NewCategorizer creates a new categorizer
func NewCategorizer(client OpenAIClient, cache Cache) *Categorizer {
	return &Categorizer{
		client: client,
		cache:  cache,
	}
}

// CategorizeItems categorizes a list of items using available categories
func (c *Categorizer) CategorizeItems(ctx context.Context, items []Item, categories []Category) (*CategorizationResult, error) {
	if len(items) == 0 {
		return &CategorizationResult{Categorizations: []ItemCategorization{}}, nil
	}

	result := &CategorizationResult{
		Categorizations: make([]ItemCategorization, 0, len(items)),
	}

	// Build category map for quick lookup
	categoryMap := make(map[string]Category)
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}

	// Separate cached and uncached items
	var uncachedItems []Item
	for _, item := range items {
		normalizedName := c.normalizeItemName(item.Name)

		// Check cache
		if categoryID, found := c.cache.Get(normalizedName); found {
			// Use cached categorization
			cat := categoryMap[categoryID]
			result.Categorizations = append(result.Categorizations, ItemCategorization{
				ItemName:     item.Name,
				CategoryID:   categoryID,
				CategoryName: cat.Name,
				Confidence:   1.0, // 100% confidence for cached items
			})
		} else {
			uncachedItems = append(uncachedItems, item)
		}
	}

	// If all items were cached, return early
	if len(uncachedItems) == 0 {
		return result, nil
	}

	// Call OpenAI for uncached items
	openAIResult, err := c.callOpenAI(ctx, uncachedItems, categories)
	if err != nil {
		return nil, fmt.Errorf("OpenAI categorization failed: %w", err)
	}

	// Process OpenAI results
	for _, cat := range openAIResult.Categorizations {
		// Cache the result
		normalizedName := c.normalizeItemName(cat.ItemName)
		c.cache.Set(normalizedName, cat.CategoryID)

		// Add to results
		result.Categorizations = append(result.Categorizations, cat)
	}

	return result, nil
}

// callOpenAI makes the actual API call to OpenAI
func (c *Categorizer) callOpenAI(ctx context.Context, items []Item, categories []Category) (*CategorizationResult, error) {
	prompt := c.buildPrompt(items, categories)

	request := ChatCompletionRequest{
		Model:       "gpt-4o", // Using GPT-4o for better performance and lower cost
		Temperature: 0.1,      // Low temperature for consistent categorization
		ResponseFormat: &ResponseFormat{
			Type: "json_object",
		},
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that categorizes shopping items into appropriate categories. Always respond with valid JSON.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	response, err := c.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse JSON response
	var result CategorizationResult
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	return &result, nil
}

// buildPrompt creates the prompt for OpenAI
func (c *Categorizer) buildPrompt(items []Item, categories []Category) string {
	var itemsList strings.Builder
	for i, item := range items {
		itemsList.WriteString(fmt.Sprintf("%d. %s - $%.2f\n", i+1, item.Name, item.Price))
	}

	var categoriesList strings.Builder
	for _, cat := range categories {
		categoriesList.WriteString(fmt.Sprintf("- %s (ID: %s)\n", cat.Name, cat.ID))
	}

	prompt := fmt.Sprintf(`Please categorize the following Walmart items into the most appropriate categories.

Items to categorize:
%s

Available categories:
%s

IMPORTANT Instructions:
1. Match each item to the MOST appropriate category
2. Distinguish between different types of items:
   - "Groceries" should be used ONLY for food items (milk, bread, meat, produce, snacks, beverages)
   - "Home & Garden" should be used for cleaning supplies, paper products (paper towels, toilet paper), laundry detergent, trash bags, and home maintenance items
   - "Personal Care" should be used for toiletries like shampoo, deodorant, toothpaste, soap, cosmetics
   - "Health & Wellness" for vitamins, medicine, first aid
3. Do NOT put non-food items in Groceries even if purchased at a grocery store
4. Consider the item name carefully - "paper towels" is Home & Garden, not Groceries
5. Provide a confidence score (0.0 to 1.0) for each categorization

Return the result as a JSON object with this structure:
{
  "categorizations": [
    {
      "item_name": "exact item name",
      "category_id": "category ID",
      "category_name": "category name",
      "confidence": 0.95
    }
  ]
}`, itemsList.String(), categoriesList.String())

	return prompt
}

// normalizeItemName normalizes an item name for cache key
func (c *Categorizer) normalizeItemName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
