// Package clients provides centralized client initialization with dependency injection.
//
// This package eliminates duplicated client setup code across commands by providing
// a single point of initialization for all external service clients.
//
// Example usage:
//
//	cfg := config.LoadOrEnv()
//	clients, err := clients.NewClients(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use clients.Monarch, clients.Categorizer, etc.
package clients

import (
	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
)

// Clients holds all initialized service clients
type Clients struct {
	Monarch     *monarch.Client
	Categorizer *categorizer.Categorizer
}

// NewClients initializes all service clients from configuration
// Returns error if required API keys are missing or client initialization fails
func NewClients(cfg *config.Config) (*Clients, error) {
	// Validate and get API keys
	monarchToken, openaiKey, err := cfg.MustGetAPIKeys()
	if err != nil {
		return nil, err
	}

	// Initialize Monarch client
	monarchClient, err := monarch.NewClientWithToken(monarchToken)
	if err != nil {
		return nil, err
	}

	// Initialize OpenAI client for categorization
	model := cfg.OpenAI.Model
	if model == "" {
		model = "gpt-4o" // Default model
	}

	openaiClient := categorizer.NewRealOpenAIClient(openaiKey)
	cache := categorizer.NewMemoryCache()
	cat := categorizer.NewCategorizer(openaiClient, cache)

	return &Clients{
		Monarch:     monarchClient,
		Categorizer: cat,
	}, nil
}
