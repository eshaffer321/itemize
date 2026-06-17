package clients

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	anthropicclient "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients/anthropic"
	openaiclient "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients/openai"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
)

const (
	providerOpenAI    = "openai"
	providerAnthropic = "anthropic"
)

type Clients struct {
	Monarch     *monarch.Client
	Categorizer *categorizer.Categorizer
}

func NewClients(cfg *config.Config) (*Clients, error) {
	monarchToken := cfg.GetAPIKey(cfg.Monarch.APIKey, "MONARCH_TOKEN")

	mClient, err := monarch.NewClientWithToken(monarchToken)
	if err != nil {
		return nil, err
	}

	chatClient, model, err := newChatClient(cfg, slog.Default())
	if err != nil {
		return nil, err
	}
	cat := categorizer.NewCategorizer(chatClient, categorizer.NewMemoryCache(), model)

	return &Clients{
		Monarch:     mClient,
		Categorizer: cat,
	}, nil
}

// newChatClient picks the configured LLM backend and returns a ChatClient plus
// the model string to hand to the categorizer.
//
// Selection rules:
//  1. cfg.Categorizer.Provider == "openai" or "anthropic" — explicit wins; key
//     for that provider must be set.
//  2. Otherwise auto-detect from which API key is set. If both keys are set,
//     OpenAI is preferred (keeps existing behavior) and a warning is logged
//     suggesting CATEGORIZER_PROVIDER for explicitness.
//  3. If no key is set, return an error.
func newChatClient(cfg *config.Config, logger *slog.Logger) (categorizer.ChatClient, string, error) {
	openKey := cfg.GetAPIKey(cfg.OpenAI.APIKey, "OPENAI_API_KEY", "OPENAI_APIKEY")
	anthKey := cfg.GetAPIKey(cfg.Anthropic.APIKey, "ANTHROPIC_API_KEY", "CLAUDE_API_KEY")
	provider := strings.ToLower(strings.TrimSpace(cfg.Categorizer.Provider))

	switch provider {
	case providerOpenAI:
		if openKey == "" {
			return nil, "", errMissingKey(providerOpenAI)
		}
		return openaiclient.NewClient(openKey), cfg.OpenAI.Model, nil
	case providerAnthropic:
		if anthKey == "" {
			return nil, "", errMissingKey(providerAnthropic)
		}
		return anthropicclient.NewClient(anthKey), cfg.Anthropic.Model, nil
	case "":
		// auto-detect
	default:
		return nil, "", fmt.Errorf("unknown categorizer provider %q (valid: openai, anthropic)", provider)
	}

	hasOpen := openKey != ""
	hasAnth := anthKey != ""
	switch {
	case hasOpen && !hasAnth:
		return openaiclient.NewClient(openKey), cfg.OpenAI.Model, nil
	case hasAnth && !hasOpen:
		return anthropicclient.NewClient(anthKey), cfg.Anthropic.Model, nil
	case hasOpen && hasAnth:
		logger.Warn("both OPENAI_API_KEY and ANTHROPIC_API_KEY are set; defaulting to openai. Set CATEGORIZER_PROVIDER=anthropic to pick Claude.")
		return openaiclient.NewClient(openKey), cfg.OpenAI.Model, nil
	default:
		return nil, "", errNoLLMKeyConfigured()
	}
}

func errMissingKey(provider string) error {
	switch provider {
	case providerOpenAI:
		return fmt.Errorf("CATEGORIZER_PROVIDER=openai but OPENAI_API_KEY is not set")
	case providerAnthropic:
		return fmt.Errorf("CATEGORIZER_PROVIDER=anthropic but ANTHROPIC_API_KEY (or CLAUDE_API_KEY) is not set")
	default:
		return fmt.Errorf("missing API key for provider %q", provider)
	}
}

func errNoLLMKeyConfigured() error {
	return fmt.Errorf("no LLM API key configured — set OPENAI_API_KEY or ANTHROPIC_API_KEY")
}
