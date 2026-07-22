package app

import (
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/ai"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
)

func buildAIService(cfg config.Config) (*ai.Service, error) {
	if cfg.AIProvider == "" || cfg.AIProvider == "disabled" {
		return nil, nil
	}
	adapterOptions := ai.AdapterOptions{
		BaseURL: cfg.AIBaseURL, Model: cfg.AIModel, APIKey: cfg.AIAPIKey,
		Timeout: cfg.AITimeout, MaxOutputBytes: cfg.AIMaxOutputBytes,
	}
	var provider ai.Provider
	var err error
	switch cfg.AIProvider {
	case "ollama":
		provider, err = ai.NewOllamaProvider(adapterOptions)
	case "openai":
		provider, err = ai.NewOpenAIProvider(adapterOptions)
	default:
		return nil, fmt.Errorf("unsupported AI provider %q", cfg.AIProvider)
	}
	if err != nil {
		return nil, fmt.Errorf("configure optional AI provider: %w", err)
	}
	service, err := ai.NewService(ai.Options{
		Provider: provider, Timeout: cfg.AITimeout,
		MaxInputBytes: cfg.AIMaxInputBytes, MaxOutputBytes: cfg.AIMaxOutputBytes,
		MaxContextItems: cfg.AIMaxContextItems, MaxDuplicateCandidates: cfg.AIMaxDuplicateCandidates,
		MaxConcurrency: cfg.AIMaxConcurrency, SupportedLocales: cfg.SupportedLocales,
		DefaultLocale: cfg.DefaultLocale,
	})
	if err != nil {
		return nil, fmt.Errorf("configure optional AI service: %w", err)
	}
	return service, nil
}

func configuredAILimits(cfg config.Config) ai.Limits {
	return ai.Limits{
		MaxInputBytes: cfg.AIMaxInputBytes, MaxOutputBytes: cfg.AIMaxOutputBytes,
		MaxContextItems: cfg.AIMaxContextItems, MaxDuplicateCandidates: cfg.AIMaxDuplicateCandidates,
	}
}
