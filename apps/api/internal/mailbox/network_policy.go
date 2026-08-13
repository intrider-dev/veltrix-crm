package mailbox

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

var (
	ErrEndpointRejected    = errors.New("mail endpoint is not allowed")
	ErrEndpointUnavailable = errors.New("mail endpoint is unavailable")
	ErrIMAPOperation       = errors.New("IMAP operation failed")
	ErrMessageTooLarge     = errors.New("mail message exceeds configured limit")
	ErrMalformedMessage    = errors.New("mail message is malformed")
)

type EndpointPolicy struct {
	Resolver     *net.Resolver
	DialTimeout  time.Duration
	AllowPrivate bool
	IMAPPorts    map[int]struct{}
	SMTPPorts    map[int]struct{}
}

func DefaultEndpointPolicy() EndpointPolicy {
	return EndpointPolicy{
		Resolver: net.DefaultResolver, DialTimeout: 10 * time.Second,
		IMAPPorts: map[int]struct{}{143: {}, 993: {}},
		SMTPPorts: map[int]struct{}{465: {}, 587: {}},
	}
}

func (policy EndpointPolicy) Validate(host string, port int, protocol string) error {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 || strings.ContainsAny(host, ":/@?#\\") ||
		!hostnamePattern.MatchString(host) || strings.Contains(host, "..") {
		return ErrEndpointRejected
	}
	if address := net.ParseIP(host); address != nil && !endpointIPAllowed(address, policy.AllowPrivate) {
		return ErrEndpointRejected
	}
	ports := policy.IMAPPorts
	if protocol == "smtp" {
		ports = policy.SMTPPorts
	} else if protocol != "imap" {
		return ErrEndpointRejected
	}
	if _, ok := ports[port]; !ok {
		return ErrEndpointRejected
	}
	return nil
}

func (policy EndpointPolicy) DialContext(ctx context.Context, host string, port int, protocol string) (net.Conn, error) {
	if err := policy.Validate(host, port, protocol); err != nil {
		return nil, err
	}
	resolver := policy.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, ErrEndpointUnavailable
	}
	allowed := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := net.IP(address.AsSlice())
		if endpointIPAllowed(ip, policy.AllowPrivate) {
			allowed = append(allowed, ip)
		}
	}
	if len(allowed) == 0 {
		return nil, ErrEndpointRejected
	}
	sort.Slice(allowed, func(i, j int) bool { return allowed[i].String() < allowed[j].String() })
	timeout := policy.DialTimeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout, KeepAlive: -1}
	var lastErr error
	for _, address := range allowed {
		connection, dialErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	_ = lastErr // Preserve no provider endpoint details in returned/logged errors.
	return nil, ErrEndpointUnavailable
}

func endpointIPAllowed(ip net.IP, allowPrivate bool) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if !allowPrivate && ip.IsPrivate() {
		return false
	}
	// Cloud instance metadata is never a valid mail endpoint, even when a
	// deployment explicitly permits private corporate ranges.
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return false
	}
	return true
}

func tlsForHost(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}
