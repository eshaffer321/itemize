package categorizer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOpenAIClient for testing
type MockOpenAIClient struct {
	mock.Mock
}

func (m *MockOpenAIClient) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error) {
	args := m.Called(ctx, request)
	if response := args.Get(0); response != nil {
		return response.(*ChatCompletionResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockCache for testing
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string) (string, bool) {
	args := m.Called(key)
	return args.String(0), args.Bool(1)
}

func (m *MockCache) Set(key string, value string) {
	m.Called(key, value)
}

func TestCategorizer_CategorizeItems_Success(t *testing.T) {
	ctx := context.Background()
	
	mockClient := new(MockOpenAIClient)
	mockCache := new(MockCache)
	
	categorizer := NewCategorizer(mockClient, mockCache)
	
	// Test data
	items := []Item{
		{Name: "Great Value Milk", Price: 3.99},
		{Name: "Bounty Paper Towels", Price: 15.99},
		{Name: "iPhone Charger", Price: 19.99},
	}
	
	categories := []Category{
		{ID: "cat_1", Name: "Groceries"},
		{ID: "cat_2", Name: "Household"},
		{ID: "cat_3", Name: "Electronics"},
		{ID: "cat_4", Name: "Clothing"},
	}
	
	// Mock cache misses
	mockCache.On("Get", "great value milk").Return("", false)
	mockCache.On("Get", "bounty paper towels").Return("", false)
	mockCache.On("Get", "iphone charger").Return("", false)
	
	// Expected OpenAI response
	openAIResponse := CategorizationResult{
		Categorizations: []ItemCategorization{
			{ItemName: "Great Value Milk", CategoryID: "cat_1", CategoryName: "Groceries", Confidence: 0.95},
			{ItemName: "Bounty Paper Towels", CategoryID: "cat_2", CategoryName: "Household", Confidence: 0.90},
			{ItemName: "iPhone Charger", CategoryID: "cat_3", CategoryName: "Electronics", Confidence: 0.98},
		},
	}
	
	responseJSON, _ := json.Marshal(openAIResponse)
	
	// Mock OpenAI call
	mockClient.On("CreateChatCompletion", ctx, mock.MatchedBy(func(req ChatCompletionRequest) bool {
		// Verify the request contains the right model and has a system message
		return req.Model == "gpt-4o" && 
			len(req.Messages) > 0 &&
			req.ResponseFormat != nil &&
			req.ResponseFormat.Type == "json_object"
	})).Return(&ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Content: string(responseJSON),
				},
			},
		},
	}, nil)
	
	// Mock cache sets
	mockCache.On("Set", "great value milk", "cat_1").Return()
	mockCache.On("Set", "bounty paper towels", "cat_2").Return()
	mockCache.On("Set", "iphone charger", "cat_3").Return()
	
	// Execute
	result, err := categorizer.CategorizeItems(ctx, items, categories)
	
	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Categorizations, 3)
	
	assert.Equal(t, "Great Value Milk", result.Categorizations[0].ItemName)
	assert.Equal(t, "cat_1", result.Categorizations[0].CategoryID)
	assert.Equal(t, "Groceries", result.Categorizations[0].CategoryName)
	
	mockClient.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestCategorizer_CategorizeItems_WithCache(t *testing.T) {
	ctx := context.Background()
	
	mockClient := new(MockOpenAIClient)
	mockCache := new(MockCache)
	
	categorizer := NewCategorizer(mockClient, mockCache)
	
	// Test data
	items := []Item{
		{Name: "Great Value Milk", Price: 3.99},
		{Name: "Bounty Paper Towels", Price: 15.99},
	}
	
	categories := []Category{
		{ID: "cat_1", Name: "Groceries"},
		{ID: "cat_2", Name: "Household"},
	}
	
	// Mock cache hits
	mockCache.On("Get", "great value milk").Return("cat_1", true)
	mockCache.On("Get", "bounty paper towels").Return("cat_2", true)
	
	// Execute - should not call OpenAI
	result, err := categorizer.CategorizeItems(ctx, items, categories)
	
	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Categorizations, 2)
	
	assert.Equal(t, "Great Value Milk", result.Categorizations[0].ItemName)
	assert.Equal(t, "cat_1", result.Categorizations[0].CategoryID)
	assert.Equal(t, "Groceries", result.Categorizations[0].CategoryName)
	assert.Equal(t, float64(1.0), result.Categorizations[0].Confidence) // Cached items have 100% confidence
	
	// Verify OpenAI was NOT called
	mockClient.AssertNotCalled(t, "CreateChatCompletion")
	mockCache.AssertExpectations(t)
}

func TestCategorizer_CategorizeItems_PartialCache(t *testing.T) {
	ctx := context.Background()
	
	mockClient := new(MockOpenAIClient)
	mockCache := new(MockCache)
	
	categorizer := NewCategorizer(mockClient, mockCache)
	
	// Test data
	items := []Item{
		{Name: "Great Value Milk", Price: 3.99},    // Cached
		{Name: "iPhone Charger", Price: 19.99},      // Not cached
	}
	
	categories := []Category{
		{ID: "cat_1", Name: "Groceries"},
		{ID: "cat_3", Name: "Electronics"},
	}
	
	// Mock cache - one hit, one miss
	mockCache.On("Get", "great value milk").Return("cat_1", true)
	mockCache.On("Get", "iphone charger").Return("", false)
	
	// Expected OpenAI response for uncached item only
	openAIResponse := CategorizationResult{
		Categorizations: []ItemCategorization{
			{ItemName: "iPhone Charger", CategoryID: "cat_3", CategoryName: "Electronics", Confidence: 0.98},
		},
	}
	
	responseJSON, _ := json.Marshal(openAIResponse)
	
	// Mock OpenAI call for uncached item
	mockClient.On("CreateChatCompletion", ctx, mock.MatchedBy(func(req ChatCompletionRequest) bool {
		// Should only include uncached item
		return req.Model == "gpt-4o"
	})).Return(&ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Content: string(responseJSON),
				},
			},
		},
	}, nil)
	
	// Mock cache set for new item
	mockCache.On("Set", "iphone charger", "cat_3").Return()
	
	// Execute
	result, err := categorizer.CategorizeItems(ctx, items, categories)
	
	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Categorizations, 2)
	
	// Check cached item
	assert.Equal(t, "Great Value Milk", result.Categorizations[0].ItemName)
	assert.Equal(t, float64(1.0), result.Categorizations[0].Confidence)
	
	// Check newly categorized item
	assert.Equal(t, "iPhone Charger", result.Categorizations[1].ItemName)
	assert.Equal(t, float64(0.98), result.Categorizations[1].Confidence)
	
	mockClient.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestCategorizer_CategorizeItems_EmptyItems(t *testing.T) {
	ctx := context.Background()
	
	mockClient := new(MockOpenAIClient)
	mockCache := new(MockCache)
	
	categorizer := NewCategorizer(mockClient, mockCache)
	
	// Test with empty items
	result, err := categorizer.CategorizeItems(ctx, []Item{}, []Category{{ID: "cat_1", Name: "Groceries"}})
	
	// Should return empty result without error
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Categorizations, 0)
	
	// Verify OpenAI was NOT called
	mockClient.AssertNotCalled(t, "CreateChatCompletion")
}

func TestCategorizer_CategorizeItems_OpenAIError(t *testing.T) {
	ctx := context.Background()
	
	mockClient := new(MockOpenAIClient)
	mockCache := new(MockCache)
	
	categorizer := NewCategorizer(mockClient, mockCache)
	
	items := []Item{
		{Name: "Test Item", Price: 10.00},
	}
	
	categories := []Category{
		{ID: "cat_1", Name: "Groceries"},
	}
	
	// Mock cache miss
	mockCache.On("Get", "test item").Return("", false)
	
	// Mock OpenAI error
	mockClient.On("CreateChatCompletion", ctx, mock.Anything).Return(nil, assert.AnError)
	
	// Execute
	result, err := categorizer.CategorizeItems(ctx, items, categories)
	
	// Verify error handling
	assert.Error(t, err)
	assert.Nil(t, result)
	
	mockClient.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestCategorizer_BuildPrompt(t *testing.T) {
	categorizer := &Categorizer{}
	
	items := []Item{
		{Name: "Milk", Price: 3.99},
		{Name: "Bread", Price: 2.50},
	}
	
	categories := []Category{
		{ID: "cat_1", Name: "Groceries"},
		{ID: "cat_2", Name: "Electronics"},
	}
	
	prompt := categorizer.buildPrompt(items, categories)
	
	// Verify prompt contains key information
	assert.Contains(t, prompt, "Milk")
	assert.Contains(t, prompt, "$3.99")
	assert.Contains(t, prompt, "Bread")
	assert.Contains(t, prompt, "$2.50")
	assert.Contains(t, prompt, "Groceries")
	assert.Contains(t, prompt, "Electronics")
	assert.Contains(t, prompt, "cat_1")
	assert.Contains(t, prompt, "cat_2")
}

func TestCategorizer_NormalizeItemName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Great Value Milk", "great value milk"},
		{"  BREAD  ", "bread"},
		{"iPhone-Charger", "iphone-charger"},
		{"Paper Towels (6 pack)", "paper towels (6 pack)"},
	}
	
	categorizer := &Categorizer{}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := categorizer.normalizeItemName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}