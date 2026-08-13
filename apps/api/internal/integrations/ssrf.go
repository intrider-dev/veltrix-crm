package integrations

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type DNSResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type URLValidator struct {
	Resolver          DNSResolver
	AllowHTTP         bool
	AllowedPorts      map[string]struct{}
	ResolutionTimeout time.Duration
}

type ResolvedEndpoint struct {
	URL       *url.URL
	Addresses []netip.Addr
	Port      string
}

func (validator URLValidator) Validate(ctx context.Context, rawURL string) (ResolvedEndpoint, error) {
	if len(rawURL) < 1 || len(rawURL) > 2048 || strings.ContainsAny(rawURL, "\r\n\x00") {
		return ResolvedEndpoint{}, errors.New("invalid webhook URL")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ResolvedEndpoint{}, errors.New("invalid webhook URL")
	}
	if parsed.Scheme != "https" && !(validator.AllowHTTP && parsed.Scheme == "http") {
		return ResolvedEndpoint{}, errors.New("webhook URL scheme is not allowed")
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return ResolvedEndpoint{}, errors.New("webhook URL host is not allowed")
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil || port == "0" {
		return ResolvedEndpoint{}, errors.New("webhook URL port is invalid")
	}
	allowedPorts := validator.AllowedPorts
	if len(allowedPorts) == 0 {
		allowedPorts = map[string]struct{}{"443": {}}
		if validator.AllowHTTP {
			allowedPorts["80"] = struct{}{}
		}
	}
	if _, allowed := allowedPorts[port]; !allowed {
		return ResolvedEndpoint{}, errors.New("webhook URL port is not allowed")
	}
	resolver := validator.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	timeout := validator.ResolutionTimeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	resolveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addresses, err := resolver.LookupNetIP(resolveCtx, "ip", hostname)
	if err != nil || len(addresses) == 0 || len(addresses) > 16 {
		return ResolvedEndpoint{}, errors.New("webhook URL host could not be resolved safely")
	}
	for _, address := range addresses {
		if !publicAddress(address) {
			return ResolvedEndpoint{}, errors.New("webhook URL resolves to a non-public address")
		}
	}
	return ResolvedEndpoint{URL: parsed, Addresses: append([]netip.Addr(nil), addresses...), Port: port}, nil
}

var blockedNetworks = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "fc00::/7", "fe80::/10", "2001:db8::/32", "ff00::/8",
)

func publicAddress(address netip.Addr) bool {
	if !address.IsValid() {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedNetworks {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParsePrefix(value))
	}
	return result
}

type SafeWebhookClient struct {
	Validator       URLValidator
	MaxResponseBody int64
	UserAgent       string
}

type WebhookResponse struct {
	StatusCode int
	Summary    string
}

func (client SafeWebhookClient) Post(ctx context.Context, rawURL string, headers http.Header, body []byte, timeout time.Duration) (WebhookResponse, error) {
	if len(body) > 262144 {
		return WebhookResponse{}, errors.New("webhook request body exceeds limit")
	}
	endpoint, err := client.Validator.Validate(ctx, rawURL)
	if err != nil {
		return WebhookResponse{}, err
	}
	if timeout < time.Second || timeout > 30*time.Second {
		timeout = 10 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.URL.String(), bytes.NewReader(body))
	if err != nil {
		return WebhookResponse{}, err
	}
	request.Header = headers.Clone()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", client.UserAgent)
	address := endpoint.Addresses[0].String()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}
	transport := &http.Transport{
		Proxy:                 nil,
		DisableKeepAlives:     true,
		MaxIdleConns:          0,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(dialCtx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(dialCtx, network, net.JoinHostPort(address, endpoint.Port))
		},
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("webhook redirects are disabled")
		},
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return WebhookResponse{}, err
	}
	defer response.Body.Close()
	limit := client.MaxResponseBody
	if limit <= 0 || limit > 16384 {
		limit = 4096
	}
	buffer, _ := io.ReadAll(io.LimitReader(response.Body, limit))
	summary := sanitizeResponseSummary(string(buffer))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return WebhookResponse{StatusCode: response.StatusCode, Summary: summary}, fmt.Errorf("webhook endpoint returned status %d", response.StatusCode)
	}
	return WebhookResponse{StatusCode: response.StatusCode, Summary: summary}, nil
}

const maxWebhookResponseSummaryBytes = 2048

var responseSummaryRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----.*`), `[private key redacted]`},
	{regexp.MustCompile(`(?i)("(?:authorization|api[_-]?key|access[_-]?token|refresh[_-]?token|session[_-]?token|cookie|password|secret)"\s*:\s*")[^"]*(")`), `${1}[redacted]${2}`},
	{regexp.MustCompile(`(?i)\b(authorization)\s*:\s*(?:(?:Bearer|Basic)\s+)?[A-Za-z0-9._~+/=-]{3,}`), `${1}: [redacted]`},
	{regexp.MustCompile(`(?i)\b(authorization|api[_-]?key|access[_-]?token|refresh[_-]?token|session[_-]?token|cookie|password|secret)(["']?\s*[:=]\s*["']?)[^\s,"';&}]{3,}`), `${1}${2}[redacted]`},
	{regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}\b`), `${1} [redacted]`},
	{regexp.MustCompile(`(?i)\bwhsec_[A-Za-z0-9_-]{8,}\b`), `whsec_[redacted]`},
	{regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`), `[email redacted]`},
	{regexp.MustCompile(`\+?[0-9][0-9 ()-]{7,}[0-9]`), `[phone redacted]`},
}

func sanitizeResponseSummary(value string) string {
	value = strings.Map(func(character rune) rune {
		if character == '\r' || character == '\n' || character == '\t' {
			return ' '
		}
		if character < 0x20 || character == 0x7f {
			return -1
		}
		return character
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	for _, redactor := range responseSummaryRedactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	if len(value) > maxWebhookResponseSummaryBytes {
		value = value[:maxWebhookResponseSummaryBytes]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}
