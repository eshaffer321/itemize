package providers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Registry manages all registered providers
type Registry struct {
	providers map[string]OrderProvider
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewRegistry creates a new provider registry
func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		providers: make(map[string]OrderProvider),
		logger:    logger,
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(provider OrderProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}
	
	r.providers[name] = provider
	r.logger.Info("registered provider",
		slog.String("provider", name),
		slog.String("display_name", provider.DisplayName()),
		slog.Bool("supports_tips", provider.SupportsDeliveryTips()),
		slog.Bool("supports_refunds", provider.SupportsRefunds()),
	)
	
	return nil
}

// Get returns a provider by name
func (r *Registry) Get(name string) (OrderProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	
	return provider, nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// GetAll returns all registered providers
func (r *Registry) GetAll() []OrderProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	providers := make([]OrderProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// HealthCheck runs health checks on all providers
func (r *Registry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for name, provider := range r.providers {
		wg.Add(1)
		go func(n string, p OrderProvider) {
			defer wg.Done()
			err := p.HealthCheck(ctx)
			mu.Lock()
			results[n] = err
			mu.Unlock()
			
			if err != nil {
				r.logger.Error("provider health check failed",
					slog.String("provider", n),
					slog.String("error", err.Error()),
				)
			} else {
				r.logger.Debug("provider health check passed",
					slog.String("provider", n),
				)
			}
		}(name, provider)
	}
	
	wg.Wait()
	return results
}

// ProcessAll runs a function on all providers
func (r *Registry) ProcessAll(ctx context.Context, fn func(context.Context, OrderProvider) error) error {
	r.mu.RLock()
	providers := make([]OrderProvider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()
	
	for _, provider := range providers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(ctx, provider); err != nil {
				r.logger.Error("provider processing failed",
					slog.String("provider", provider.Name()),
					slog.String("error", err.Error()),
				)
				// Continue with other providers
			}
		}
	}
	
	return nil
}