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

func TestLoadBootstrapDoesNotRequireApplicationEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_ADMIN_URL", "postgres://admin@postgres/crm")
	t.Setenv("APP_DB_PASSWORD", "app-password")
	t.Setenv("DEMO_SEED", "false")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_BASE64", "")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_ID", "")

	cfg, err := LoadBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IdentityKeyBase64 != "" || cfg.IdentityKeyID != "" {
		t.Fatalf("bootstrap loaded application encryption configuration: %+v", cfg)
	}
}

func TestLoadBootstrapRejectsProductionDemoSeedAndMissingCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_ADMIN_URL", "postgres://admin@postgres/crm")
	t.Setenv("APP_DB_PASSWORD", "app-password")
	t.Setenv("DEMO_SEED", "true")
	if _, err := LoadBootstrap(); err == nil || !strings.Contains(err.Error(), "DEMO_SEED") {
		t.Fatalf("error = %v, want production seed rejection", err)
	}

	t.Setenv("DEMO_SEED", "false")
	t.Setenv("DATABASE_ADMIN_URL", "")
	if _, err := LoadBootstrap(); err == nil || !strings.Contains(err.Error(), "DATABASE_ADMIN_URL") {
		t.Fatalf("error = %v, want missing bootstrap credentials", err)
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

func TestBrokerLoadDefaultsToPostgres(t *testing.T) {
	setMinimumEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrokerMode != "postgres" || len(cfg.BrokerJobKinds()) != 0 {
		t.Fatalf("unexpected broker defaults: mode=%q jobs=%v", cfg.BrokerMode, cfg.BrokerJobKinds())
	}
}

func TestBrokerLoadRequiresTLSInProduction(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_BASE64", "configured")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_ID", "production-v1")
	t.Setenv("BROKER_MODE", "combined")
	t.Setenv("KAFKA_BROKERS", "kafka.internal:9092")
	t.Setenv("KAFKA_USERNAME", "publisher")
	t.Setenv("KAFKA_PASSWORD", "secret")
	t.Setenv("KAFKA_TLS", "false")
	t.Setenv("RABBITMQ_HOST", "rabbit.internal")
	t.Setenv("RABBITMQ_USERNAME", "publisher")
	t.Setenv("RABBITMQ_PASSWORD", "secret")
	t.Setenv("RABBITMQ_TLS", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "KAFKA_TLS") {
		t.Fatalf("error = %v, want Kafka TLS rejection", err)
	}

	t.Setenv("KAFKA_TLS", "true")
	t.Setenv("RABBITMQ_TLS", "false")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "RABBITMQ_TLS") {
		t.Fatalf("error = %v, want RabbitMQ TLS rejection", err)
	}
}

func TestBrokerLoadRequiresKafkaAuthenticationInProduction(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_BASE64", "configured")
	t.Setenv("IDENTITY_ENCRYPTION_KEY_ID", "production-v1")
	t.Setenv("BROKER_MODE", "kafka")
	t.Setenv("KAFKA_BROKERS", "kafka.internal:9093")
	t.Setenv("KAFKA_TLS", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "KAFKA_USERNAME") {
		t.Fatalf("error = %v, want Kafka authentication rejection", err)
	}
}

func TestBrokerLoadAcceptsBoundedDevelopmentProfile(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("BROKER_MODE", "combined")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")
	t.Setenv("RABBITMQ_HOST", "rabbitmq")
	t.Setenv("RABBITMQ_USERNAME", "publisher")
	t.Setenv("RABBITMQ_PASSWORD", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.BrokerJobKinds(), ","); got != "broker.kafka.publish,broker.rabbitmq.publish" {
		t.Fatalf("broker jobs = %q", got)
	}
}

func TestBrokerLoadRejectsCredentialsInEndpoints(t *testing.T) {
	setMinimumEnvironment(t)
	t.Setenv("BROKER_MODE", "kafka")
	t.Setenv("KAFKA_BROKERS", "sasl://user:secret@kafka:9092")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("error = %v, want credential-bearing Kafka address rejection", err)
	}

	setMinimumEnvironment(t)
	t.Setenv("BROKER_MODE", "rabbitmq")
	t.Setenv("RABBITMQ_HOST", "user:secret@rabbitmq")
	t.Setenv("RABBITMQ_USERNAME", "publisher")
	t.Setenv("RABBITMQ_PASSWORD", "secret")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "RABBITMQ_HOST") {
		t.Fatalf("error = %v, want RabbitMQ URL syntax rejection", err)
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
	t.Setenv("BROKER_MODE", "")
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_USERNAME", "")
	t.Setenv("KAFKA_PASSWORD", "")
	t.Setenv("KAFKA_TLS", "")
	t.Setenv("KAFKA_AUTO_CREATE_TOPICS", "")
	t.Setenv("RABBITMQ_HOST", "")
	t.Setenv("RABBITMQ_USERNAME", "")
	t.Setenv("RABBITMQ_PASSWORD", "")
	t.Setenv("RABBITMQ_TLS", "")
}
