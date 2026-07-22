package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadStorageDefaultsToLocal(t *testing.T) {
	setMinimumEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageBackend != "local" || cfg.MaxUploadBytes != 10<<20 {
		t.Fatalf("unexpected storage defaults: backend=%q max=%d", cfg.StorageBackend, cfg.MaxUploadBytes)
	}
}

func TestLoadRequiresCompleteS3Configuration(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("STORAGE_BACKEND", "s3")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "S3_ENDPOINT") {
		t.Fatalf("error = %v, want incomplete S3 configuration error", err)
	}
	t.Setenv("S3_ENDPOINT", "https://objects.example.invalid")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3Region != "us-east-1" || cfg.S3Bucket != "crm-attachments" {
		t.Fatalf("unexpected S3 defaults: %+v", cfg)
	}
}

func TestAILoadDefaultsToDisabledAndBounded(t *testing.T) {
	setMinimumEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIProvider != "disabled" {
		t.Fatalf("provider = %q, want disabled", cfg.AIProvider)
	}
	if cfg.AITimeout.String() != "15s" || cfg.AIMaxInputBytes != 32<<10 || cfg.AIMaxOutputBytes != 8<<10 {
		t.Fatalf("unexpected AI defaults: timeout=%s input=%d output=%d", cfg.AITimeout, cfg.AIMaxInputBytes, cfg.AIMaxOutputBytes)
	}
	if cfg.AIMaxContextItems != 50 || cfg.AIMaxDuplicateCandidates != 25 || cfg.AIMaxConcurrency != 2 || cfg.AIRequestsPerMinute != 10 {
		t.Fatalf("unexpected AI count defaults: %+v", cfg)
	}
}

func TestAILoadValidatesProviderConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		baseURL  string
		model    string
		apiKey   string
		want     string
	}{
		{name: "unknown provider", provider: "mystery", want: "AI_PROVIDER"},
		{name: "missing model", provider: "ollama", baseURL: "http://ollama:11434", want: "AI_MODEL"},
		{name: "public ollama host", provider: "ollama", baseURL: "https://models.example.test", model: "model", want: "local or private"},
		{name: "insecure openai", provider: "openai", baseURL: "http://api.example.test/v1", model: "model", apiKey: "secret", want: "HTTPS"},
		{name: "missing external key", provider: "openai", baseURL: "https://api.example.test/v1", model: "model", want: "AI_API_KEY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setMinimumEnvironment(t)
			t.Setenv("AI_PROVIDER", test.provider)
			t.Setenv("AI_BASE_URL", test.baseURL)
			t.Setenv("AI_MODEL", test.model)
			t.Setenv("AI_API_KEY", test.apiKey)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if strings.Contains(err.Error(), test.apiKey) && test.apiKey != "" {
				t.Fatalf("configuration error exposed API key: %v", err)
			}
		})
	}
}

func TestAILoadAcceptsExplicitLocalAndExternalProviders(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("AI_PROVIDER", "ollama")
	t.Setenv("AI_BASE_URL", "http://ollama:11434")
	t.Setenv("AI_MODEL", "qwen-local")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIProvider != "ollama" || cfg.AIBaseURL != "http://ollama:11434" {
		t.Fatalf("unexpected local AI configuration: %+v", cfg)
	}

	setMinimumEnvironment(t)
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_BASE_URL", "https://api.example.test/v1/")
	t.Setenv("AI_MODEL", "compatible-model")
	t.Setenv("AI_API_KEY", "server-side-secret")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIProvider != "openai" || cfg.AIBaseURL != "https://api.example.test/v1" {
		t.Fatalf("unexpected external AI configuration: %+v", cfg)
	}
}

func TestAILoadRejectsUnboundedLimits(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("AI_TIMEOUT", "61s")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_TIMEOUT") {
		t.Fatalf("error = %v, want timeout bound", err)
	}
	setMinimumEnvironment(t)
	t.Setenv("AI_MAX_INPUT_BYTES", "65537")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_MAX_INPUT_BYTES") {
		t.Fatalf("error = %v, want input bound", err)
	}
}

func TestCallsLoadDefaultsToDisabled(t *testing.T) {
	setMinimumEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CallsProvider != "disabled" || cfg.LiveKitTokenTTL != 5*time.Minute {
		t.Fatalf("unexpected calls defaults: provider=%q ttl=%s", cfg.CallsProvider, cfg.LiveKitTokenTTL)
	}
}

func TestCallsLoadValidatesLiveKitOriginAndProductionTLS(t *testing.T) {
	tests := []struct {
		name, environment, publicURL, want string
	}{
		{name: "http is not websocket", publicURL: "https://calls.example.test", want: "WS(S)"},
		{name: "path is rejected", publicURL: "wss://calls.example.test/room", want: "origin"},
		{name: "production requires tls", environment: "production", publicURL: "ws://calls.example.test", want: "WSS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setMinimumEnvironment(t)
			if test.environment != "" {
				t.Setenv("APP_ENV", test.environment)
				t.Setenv("SESSION_COOKIE_SECURE", "true")
				t.Setenv("IDENTITY_ENCRYPTION_KEY_BASE64", "configured")
				t.Setenv("IDENTITY_ENCRYPTION_KEY_ID", "production-v1")
			}
			t.Setenv("CALLS_PROVIDER", "livekit")
			t.Setenv("LIVEKIT_PUBLIC_URL", test.publicURL)
			t.Setenv("LIVEKIT_API_KEY", "key")
			t.Setenv("LIVEKIT_API_SECRET", "secret")
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCallsLoadAcceptsDevelopmentLiveKit(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("CALLS_PROVIDER", "livekit")
	t.Setenv("LIVEKIT_PUBLIC_URL", "ws://localhost:7880/")
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_TOKEN_TTL", "2m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LiveKitPublicURL != "ws://localhost:7880" || cfg.LiveKitTokenTTL != 2*time.Minute {
		t.Fatalf("unexpected calls configuration: %+v", cfg)
	}
}

func setMinimumEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://app@localhost/crm")
	t.Setenv("DEMO_SEED", "false")
	t.Setenv("SMTP_ADDR", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("STORAGE_BACKEND", "")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("AI_BASE_URL", "")
	t.Setenv("AI_MODEL", "")
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_TIMEOUT", "")
	t.Setenv("AI_MAX_INPUT_BYTES", "")
	t.Setenv("AI_MAX_OUTPUT_BYTES", "")
	t.Setenv("AI_MAX_CONTEXT_ITEMS", "")
	t.Setenv("AI_MAX_DUPLICATE_CANDIDATES", "")
	t.Setenv("AI_MAX_CONCURRENCY", "")
	t.Setenv("AI_REQUESTS_PER_MINUTE", "")
	t.Setenv("CALLS_PROVIDER", "")
	t.Setenv("LIVEKIT_PUBLIC_URL", "")
	t.Setenv("LIVEKIT_API_KEY", "")
	t.Setenv("LIVEKIT_API_SECRET", "")
	t.Setenv("LIVEKIT_TOKEN_TTL", "")
}
