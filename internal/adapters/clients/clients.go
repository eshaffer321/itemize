package clients

import (
	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
)

type Clients struct {
	Monarch     *monarch.Client
	Categorizer *categorizer.Categorizer
}

func NewClients(cfg *config.Config) (*Clients, error) {
	// Get API keys with fallback to alternative env var names
	monarchToken := cfg.GetAPIKey(cfg.Monarch.APIKey, "MONARCH_TOKEN")
	openAIKey := cfg.GetAPIKey(cfg.OpenAI.APIKey, "OPENAI_API_KEY", "OPENAI_APIKEY")

	// Initialize Monarch client
	mClient, err := monarch.NewClientWithToken(monarchToken)
	if err != nil {
		return nil, err
	}

	// Initialize OpenAI client and cache for categorizer
	openAIClient := categorizer.NewRealOpenAIClient(openAIKey)
	cache := categorizer.NewMemoryCache()
	cat := categorizer.NewCategorizer(openAIClient, cache)

	return &Clients{
		Monarch:     mClient,
		Categorizer: cat,
	}, nil
}
