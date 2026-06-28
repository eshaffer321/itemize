package categorizer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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

// Chat-completion request/response types.
// The shape mirrors OpenAI's chat-completions API and is treated as the
// shared vocabulary that adapters translate to and from. Adapters for
// non-OpenAI backends (e.g. Anthropic) consume the same types.
type ChatCompletionRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Temperature     *float64        `json:"temperature,omitempty"`
	ReasoningEffort *string         `json:"reasoning_effort,omitempty"`
	ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`
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

// ChatClient is the LLM-backend interface the categorizer depends on.
// Concrete implementations live in internal/adapters/clients/{openai,anthropic}.
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// Cache interface for category mappings
type Cache interface {
	Get(key string) (string, bool)
	Set(key string, value string)
}

// Categorizer handles item categorization using a pluggable LLM backend.
type Categorizer struct {
	client ChatClient
	cache  Cache
	Model  string
}

// NewCategorizer creates a new categorizer
func NewCategorizer(client ChatClient, cache Cache, model string) *Categorizer {
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}

	return &Categorizer{
		client: client,
		cache:  cache,
		Model:  model,
	}
}

const DefaultModel = "gpt-5.4-nano"

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

	// Call the LLM for uncached items
	llmResult, err := c.callLLM(ctx, uncachedItems, categories)
	if err != nil {
		return nil, fmt.Errorf("LLM categorization failed: %w", err)
	}

	// Build a lookup so we can validate what the LLM returned
	categoryByID := make(map[string]Category, len(categories))
	categoryByName := make(map[string]Category, len(categories))
	for _, c := range categories {
		categoryByID[c.ID] = c
		categoryByName[strings.ToLower(c.Name)] = c
	}

	// Truncate extra entries — LLMs occasionally hallucinate more categorizations
	// than items sent. Extra entries corrupt category-group detection downstream.
	llmCategorizations := llmResult.Categorizations
	if len(llmCategorizations) > len(uncachedItems) {
		llmCategorizations = llmCategorizations[:len(uncachedItems)]
	}

	// Process LLM results
	for _, cat := range llmCategorizations {
		// If the LLM returned an ID that isn't in the Monarch category list,
		// try to recover via name match before falling back to empty.
		if _, ok := categoryByID[cat.CategoryID]; !ok {
			if matched, ok := categoryByName[strings.ToLower(cat.CategoryName)]; ok {
				cat.CategoryID = matched.ID
				cat.CategoryName = matched.Name
			} else {
				// No valid match — zero out the ID so callers know to skip category update
				cat.CategoryID = ""
			}
		}

		// Only cache valid IDs so future lookups don't reuse a bad value
		if cat.CategoryID != "" {
			normalizedName := c.normalizeItemName(cat.ItemName)
			c.cache.Set(normalizedName, cat.CategoryID)
		}

		result.Categorizations = append(result.Categorizations, cat)
	}

	return result, nil
}

// Retry configuration
const (
	maxRetries = 3
	baseDelay  = 1 * time.Second
	maxDelay   = 8 * time.Second
)

// isRetryableError determines if an error is transient and worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Connection errors, timeouts - check error message
	errMsg := err.Error()
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "temporary failure") ||
		strings.Contains(errMsg, "502") ||
		strings.Contains(errMsg, "503") ||
		strings.Contains(errMsg, "504")
}

// callLLM makes the actual API call to the LLM with retry logic
func (c *Categorizer) callLLM(ctx context.Context, items []Item, categories []Category) (*CategorizationResult, error) {
	prompt := c.buildPrompt(items, categories)

	request := ChatCompletionRequest{
		Model: c.Model,
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
	if isGPT5Model(c.Model) {
		reasoningEffort := "low"
		request.ReasoningEffort = &reasoningEffort
	} else {
		temperature := 0.1
		request.Temperature = &temperature
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := c.client.CreateChatCompletion(ctx, request)
		if err != nil {
			lastErr = err
			if !isRetryableError(err) || attempt == maxRetries {
				break
			}
			// Exponential backoff: 1s, 2s, 4s
			delay := baseDelay * time.Duration(1<<(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
			continue
		}

		if len(response.Choices) == 0 {
			return nil, fmt.Errorf("no response from LLM")
		}

		// Parse JSON response
		var result CategorizationResult
		if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &result); err != nil {
			return nil, fmt.Errorf("failed to parse LLM response: %w", err)
		}

		return &result, nil
	}

	return nil, fmt.Errorf("%w after %d attempts", lastErr, maxRetries)
}

func isGPT5Model(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-5")
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

	prompt := fmt.Sprintf(`Please categorize the following items into the most appropriate categories.

Items to categorize:
%s

Available categories (use ONLY these exact IDs):
%s

IMPORTANT Instructions:
1. Match each item to the MOST appropriate category from the list above
2. You MUST use the exact category_id values shown in the list — do NOT invent IDs or use words like "Uncategorized"
3. If no category is a good fit, pick the closest one available
4. Distinguish between different types of items:
   - "Groceries" should be used ONLY for food items (milk, bread, meat, produce, snacks, beverages)
   - "Home & Garden" for cleaning supplies, paper products, laundry, trash bags, home maintenance
   - "Personal Care" for toiletries: shampoo, deodorant, toothpaste, soap, cosmetics
   - "Health & Wellness" for vitamins, medicine, first aid
5. Do NOT put non-food items in Groceries
6. Provide a confidence score (0.0 to 1.0) for each categorization

Return the result as a JSON object with this structure:
{
  "categorizations": [
    {
      "item_name": "exact item name",
      "category_id": "exact ID from the list above",
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
