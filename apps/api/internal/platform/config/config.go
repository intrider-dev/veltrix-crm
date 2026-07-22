package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
)

type Config struct {
	Environment              string
	Address                  string
	PublicURL                string
	DefaultLocale            string
	SupportedLocales         []string
	DatabaseURL              string
	DatabaseAdminURL         string
	DatabaseDispatcherURL    string
	AppDBPassword            string
	CookieSecure             bool
	SessionTTL               time.Duration
	PasswordResetTTL         time.Duration
	MFAChallengeTTL          time.Duration
	MFASetupTTL              time.Duration
	IdentityKeyID            string
	IdentityKeyBase64        string
	SMTPAddress              string
	SMTPFrom                 string
	SMTPUsername             string
	SMTPPassword             string
	SMTPRequireTLS           bool
	SMTPImplicitTLS          bool
	SMTPWriteTimeout         time.Duration
	MaxDBConnections         int32
	WorkerConcurrency        int
	AutoMigrate              bool
	DemoSeed                 bool
	DemoEmail                string
	DemoPassword             string
	UploadDir                string
	MaxUploadBytes           int64
	StorageBackend           string
	S3Endpoint               string
	S3Region                 string
	S3Bucket                 string
	S3AccessKey              string
	S3SecretKey              string
	S3SessionToken           string
	S3AllowInsecure          bool
	AIProvider               string
	AIBaseURL                string
	AIModel                  string
	AIAPIKey                 string
	AITimeout                time.Duration
	AIMaxInputBytes          int64
	AIMaxOutputBytes         int64
	AIMaxContextItems        int
	AIMaxDuplicateCandidates int
	AIMaxConcurrency         int
	AIRequestsPerMinute      int
	CallsProvider            string
	LiveKitPublicURL         string
	LiveKitAPIKey            string
	LiveKitAPISecret         string
	LiveKitTokenTTL          time.Duration
	ShutdownTimeout          time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:           value("APP_ENV", "development"),
		Address:               value("APP_ADDR", ":8080"),
		PublicURL:             strings.TrimRight(value("APP_PUBLIC_URL", "http://localhost:8080"), "/"),
		DefaultLocale:         strings.ToLower(value("APP_DEFAULT_LOCALE", brand.Config.DefaultLocale)),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		DatabaseAdminURL:      os.Getenv("DATABASE_ADMIN_URL"),
		DatabaseDispatcherURL: os.Getenv("DATABASE_DISPATCHER_URL"),
		AppDBPassword:         os.Getenv("APP_DB_PASSWORD"),
		DemoEmail:             value("DEMO_EMAIL", "admin@demo.local"),
		DemoPassword:          value("DEMO_PASSWORD", "Demo123!"),
		IdentityKeyID:         value("IDENTITY_ENCRYPTION_KEY_ID", "dev-local-v1"),
		IdentityKeyBase64:     strings.TrimSpace(os.Getenv("IDENTITY_ENCRYPTION_KEY_BASE64")),
		SMTPAddress:           strings.TrimSpace(os.Getenv("SMTP_ADDR")),
		SMTPFrom:              value("SMTP_FROM", brand.Config.ProductName+" <no-reply@demo.local>"),
		SMTPUsername:          strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:          os.Getenv("SMTP_PASSWORD"),
		UploadDir:             value("UPLOAD_DIR", "./tmp/uploads"),
		StorageBackend:        strings.ToLower(value("STORAGE_BACKEND", "local")),
		S3Endpoint:            strings.TrimSpace(os.Getenv("S3_ENDPOINT")),
		S3Region:              value("S3_REGION", "us-east-1"),
		S3Bucket:              value("S3_BUCKET", "crm-attachments"),
		S3AccessKey:           strings.TrimSpace(os.Getenv("S3_ACCESS_KEY")),
		S3SecretKey:           os.Getenv("S3_SECRET_KEY"),
		S3SessionToken:        os.Getenv("S3_SESSION_TOKEN"),
		AIProvider:            strings.ToLower(value("AI_PROVIDER", "disabled")),
		AIBaseURL:             strings.TrimRight(strings.TrimSpace(os.Getenv("AI_BASE_URL")), "/"),
		AIModel:               strings.TrimSpace(os.Getenv("AI_MODEL")),
		AIAPIKey:              os.Getenv("AI_API_KEY"),
		CallsProvider:         strings.ToLower(value("CALLS_PROVIDER", "disabled")),
		LiveKitPublicURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("LIVEKIT_PUBLIC_URL")), "/"),
		LiveKitAPIKey:         strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
		LiveKitAPISecret:      os.Getenv("LIVEKIT_API_SECRET"),
	}
	for _, locale := range strings.Split(value("APP_SUPPORTED_LOCALES", strings.Join(brand.Config.SupportedLocales, ",")), ",") {
		locale = strings.ToLower(strings.TrimSpace(locale))
		if locale == "" {
			continue
		}
		if !brand.SupportsLocale(locale) {
			return Config{}, fmt.Errorf("APP_SUPPORTED_LOCALES contains unsupported locale %q", locale)
		}
		if !contains(cfg.SupportedLocales, locale) {
			cfg.SupportedLocales = append(cfg.SupportedLocales, locale)
		}
	}
	if len(cfg.SupportedLocales) == 0 || !contains(cfg.SupportedLocales, cfg.DefaultLocale) {
		return Config{}, errors.New("APP_DEFAULT_LOCALE must be included in APP_SUPPORTED_LOCALES")
	}

	var err error
	if cfg.CookieSecure, err = boolValue("SESSION_COOKIE_SECURE", cfg.Environment == "production"); err != nil {
		return Config{}, err
	}
	if cfg.AutoMigrate, err = boolValue("AUTO_MIGRATE", cfg.Environment != "production"); err != nil {
		return Config{}, err
	}
	if cfg.DemoSeed, err = boolValue("DEMO_SEED", cfg.Environment == "development"); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = durationValue("SESSION_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.PasswordResetTTL, err = durationValue("PASSWORD_RESET_TTL", time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.MFAChallengeTTL, err = durationValue("MFA_CHALLENGE_TTL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.MFASetupTTL, err = durationValue("MFA_SETUP_TTL", 10*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.SMTPWriteTimeout, err = durationValue("SMTP_WRITE_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.SMTPRequireTLS, err = boolValue("SMTP_REQUIRE_TLS", cfg.Environment == "production"); err != nil {
		return Config{}, err
	}
	if cfg.SMTPImplicitTLS, err = boolValue("SMTP_IMPLICIT_TLS", false); err != nil {
		return Config{}, err
	}
	if cfg.S3AllowInsecure, err = boolValue("S3_ALLOW_INSECURE", false); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationValue("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.AITimeout, err = durationValue("AI_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.LiveKitTokenTTL, err = durationValue("LIVEKIT_TOKEN_TTL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.AITimeout > time.Minute {
		return Config{}, errors.New("AI_TIMEOUT must not exceed 1m")
	}
	if cfg.MaxDBConnections, err = int32Value("MAX_DB_CONNECTIONS", 8, 1, 64); err != nil {
		return Config{}, err
	}
	workerConcurrency, err := int32Value("WORKER_CONCURRENCY", 2, 1, 32)
	if err != nil {
		return Config{}, err
	}
	cfg.WorkerConcurrency = int(workerConcurrency)
	if cfg.MaxUploadBytes, err = int64Value("MAX_UPLOAD_BYTES", 10<<20, 1<<20, 100<<20); err != nil {
		return Config{}, err
	}
	if cfg.AIMaxInputBytes, err = int64Value("AI_MAX_INPUT_BYTES", 32<<10, 1<<10, 64<<10); err != nil {
		return Config{}, err
	}
	if cfg.AIMaxOutputBytes, err = int64Value("AI_MAX_OUTPUT_BYTES", 8<<10, 256, 32<<10); err != nil {
		return Config{}, err
	}
	maxContextItems, err := int32Value("AI_MAX_CONTEXT_ITEMS", 50, 1, 200)
	if err != nil {
		return Config{}, err
	}
	cfg.AIMaxContextItems = int(maxContextItems)
	maxDuplicateCandidates, err := int32Value("AI_MAX_DUPLICATE_CANDIDATES", 25, 1, 100)
	if err != nil {
		return Config{}, err
	}
	cfg.AIMaxDuplicateCandidates = int(maxDuplicateCandidates)
	maxAIConcurrency, err := int32Value("AI_MAX_CONCURRENCY", 2, 1, 16)
	if err != nil {
		return Config{}, err
	}
	cfg.AIMaxConcurrency = int(maxAIConcurrency)
	aiRequestsPerMinute, err := int32Value("AI_REQUESTS_PER_MINUTE", 10, 1, 120)
	if err != nil {
		return Config{}, err
	}
	cfg.AIRequestsPerMinute = int(aiRequestsPerMinute)

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return Config{}, errors.New("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	if cfg.SMTPAddress == "" && (cfg.SMTPUsername != "" || os.Getenv("SMTP_FROM") != "") {
		return Config{}, errors.New("SMTP_ADDR is required when SMTP credentials or SMTP_FROM are configured")
	}
	if cfg.StorageBackend != "local" && cfg.StorageBackend != "s3" {
		return Config{}, errors.New("STORAGE_BACKEND must be local or s3")
	}
	if cfg.StorageBackend == "s3" {
		if cfg.S3Endpoint == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
			return Config{}, errors.New("S3_ENDPOINT, S3_ACCESS_KEY and S3_SECRET_KEY are required for the s3 storage backend")
		}
		if cfg.Environment == "production" && cfg.S3AllowInsecure {
			return Config{}, errors.New("S3_ALLOW_INSECURE must be false in production")
		}
	}
	if err := validateAIConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateCallsConfig(cfg); err != nil {
		return Config{}, err
	}
	if cfg.Environment == "production" {
		if cfg.DemoSeed {
			return Config{}, errors.New("DEMO_SEED must be false in production")
		}
		if !cfg.CookieSecure {
			return Config{}, errors.New("SESSION_COOKIE_SECURE must be true in production")
		}
		if cfg.IdentityKeyBase64 == "" {
			return Config{}, errors.New("IDENTITY_ENCRYPTION_KEY_BASE64 is required in production")
		}
		if cfg.IdentityKeyID == "dev-local-v1" {
			return Config{}, errors.New("IDENTITY_ENCRYPTION_KEY_ID must not use the development identifier in production")
		}
	}
	if cfg.AutoMigrate && (cfg.DatabaseAdminURL == "" || cfg.AppDBPassword == "") {
		return Config{}, errors.New("AUTO_MIGRATE requires DATABASE_ADMIN_URL and APP_DB_PASSWORD")
	}
	if cfg.DatabaseDispatcherURL == "" {
		cfg.DatabaseDispatcherURL = cfg.DatabaseURL
	}
	return cfg, nil
}

func validateCallsConfig(cfg Config) error {
	switch cfg.CallsProvider {
	case "disabled":
		return nil
	case "livekit":
	default:
		return errors.New("CALLS_PROVIDER must be disabled or livekit")
	}
	if cfg.LiveKitPublicURL == "" || cfg.LiveKitAPIKey == "" || cfg.LiveKitAPISecret == "" {
		return errors.New("LIVEKIT_PUBLIC_URL, LIVEKIT_API_KEY, and LIVEKIT_API_SECRET are required when calls are enabled")
	}
	if cfg.LiveKitTokenTTL < time.Minute || cfg.LiveKitTokenTTL > 10*time.Minute {
		return errors.New("LIVEKIT_TOKEN_TTL must be between 1m and 10m")
	}
	providerURL, err := url.Parse(cfg.LiveKitPublicURL)
	if err != nil || providerURL.Host == "" || (providerURL.Scheme != "ws" && providerURL.Scheme != "wss") {
		return errors.New("LIVEKIT_PUBLIC_URL must be an absolute WS(S) URL")
	}
	if providerURL.User != nil || providerURL.RawQuery != "" || providerURL.Fragment != "" || (providerURL.Path != "" && providerURL.Path != "/") {
		return errors.New("LIVEKIT_PUBLIC_URL must be an origin without credentials, path, query, or fragment")
	}
	if cfg.Environment == "production" && providerURL.Scheme != "wss" {
		return errors.New("LIVEKIT_PUBLIC_URL must use WSS in production")
	}
	if len(cfg.LiveKitAPIKey) > 256 || strings.IndexFunc(cfg.LiveKitAPIKey, unicode.IsControl) >= 0 {
		return errors.New("LIVEKIT_API_KEY must be at most 256 characters and contain no control characters")
	}
	if cfg.LiveKitAPISecret != strings.TrimSpace(cfg.LiveKitAPISecret) || len(cfg.LiveKitAPISecret) > 4096 || strings.IndexFunc(cfg.LiveKitAPISecret, unicode.IsControl) >= 0 {
		return errors.New("LIVEKIT_API_SECRET must be at most 4096 characters and contain no control characters")
	}
	return nil
}

func validateAIConfig(cfg Config) error {
	switch cfg.AIProvider {
	case "disabled":
		return nil
	case "ollama", "openai":
	default:
		return errors.New("AI_PROVIDER must be disabled, ollama, or openai")
	}
	if cfg.AIBaseURL == "" || cfg.AIModel == "" {
		return errors.New("AI_BASE_URL and AI_MODEL are required when AI_PROVIDER is enabled")
	}
	if len(cfg.AIBaseURL) > 2048 {
		return errors.New("AI_BASE_URL is too long")
	}
	if len(cfg.AIModel) > 200 || strings.IndexFunc(cfg.AIModel, unicode.IsControl) >= 0 {
		return errors.New("AI_MODEL must be at most 200 characters and contain no control characters")
	}
	if cfg.AIAPIKey != strings.TrimSpace(cfg.AIAPIKey) || len(cfg.AIAPIKey) > 4096 || strings.IndexFunc(cfg.AIAPIKey, unicode.IsControl) >= 0 {
		return errors.New("AI_API_KEY must be at most 4096 characters and contain no control characters")
	}
	providerURL, err := url.Parse(cfg.AIBaseURL)
	if err != nil || providerURL.Host == "" || (providerURL.Scheme != "http" && providerURL.Scheme != "https") {
		return errors.New("AI_BASE_URL must be an absolute HTTP(S) URL")
	}
	if providerURL.User != nil || providerURL.RawQuery != "" || providerURL.Fragment != "" {
		return errors.New("AI_BASE_URL must not contain credentials, a query, or a fragment")
	}
	if cfg.AIProvider == "openai" {
		if providerURL.Scheme != "https" {
			return errors.New("AI_BASE_URL must use HTTPS for the external openai provider")
		}
		if strings.TrimSpace(cfg.AIAPIKey) == "" {
			return errors.New("AI_API_KEY is required for the openai provider")
		}
	} else if !isLocalAIProviderHost(providerURL.Hostname()) {
		return errors.New("AI_BASE_URL for the local ollama provider must use a local or private host")
	}
	return nil
}

func isLocalAIProviderHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") || !strings.Contains(host, ".") {
		return host != ""
	}
	address := net.ParseIP(host)
	return address != nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast())
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func value(key, fallback string) string {
	if current := strings.TrimSpace(os.Getenv(key)); current != "" {
		return current
	}
	return fallback
}

func boolValue(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func durationValue(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return parsed, nil
}

func int32Value(key string, fallback, minimum, maximum int32) (int32, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if parsed < int64(minimum) || parsed > int64(maximum) {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minimum, maximum)
	}
	return int32(parsed), nil
}

func int64Value(key string, fallback, minimum, maximum int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minimum, maximum)
	}
	return parsed, nil
}
