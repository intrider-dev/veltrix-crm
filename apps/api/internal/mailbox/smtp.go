package mailbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	messageMail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

var (
	ErrSMTPAuthentication      = errors.New("SMTP authentication failed")
	ErrSMTPEnvelopeRejected    = errors.New("SMTP envelope was rejected")
	ErrSMTPSubmissionUncertain = errors.New("SMTP submission outcome is uncertain")
)

type SMTPTransport interface {
	Send(context.Context, ConnectionConfig, string, string, string, SendInput) error
}

type EmersionSMTP struct {
	Policy            EndpointPolicy
	CommandTimeout    time.Duration
	SubmissionTimeout time.Duration
	MaxMessageBytes   int64
}

func (transport EmersionSMTP) Send(
	ctx context.Context,
	config ConnectionConfig,
	password string,
	from string,
	messageID string,
	input SendInput,
) error {
	fromAddress, err := exactAddress(from)
	if err != nil {
		return ErrMalformedMessage
	}
	recipients, err := validatedRecipients(input.Recipients)
	if err != nil {
		return err
	}
	payload, err := buildPlainMessage(*fromAddress, messageID, input)
	if err != nil {
		return err
	}
	maxMessage := transport.MaxMessageBytes
	if maxMessage <= 0 || maxMessage > MaxMessageBytes {
		maxMessage = MaxMessageBytes
	}
	if int64(len(payload)) > maxMessage {
		return ErrMessageTooLarge
	}
	client, err := transport.connect(ctx, config)
	if err != nil {
		return err
	}
	defer client.Close()
	if !client.SupportsAuth("PLAIN") {
		return ErrSMTPAuthentication
	}
	if err := client.Auth(sasl.NewPlainClient("", config.Username, password)); err != nil {
		return ErrSMTPAuthentication
	}
	if err := client.Mail(fromAddress.Address, &smtp.MailOptions{Size: int64(len(payload))}); err != nil {
		return ErrSMTPEnvelopeRejected
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient, nil); err != nil {
			return ErrSMTPEnvelopeRejected
		}
	}
	data, err := client.Data()
	if err != nil {
		return ErrSMTPEnvelopeRejected
	}
	if _, err := io.Copy(data, bytes.NewReader(payload)); err != nil {
		_ = data.Close()
		return ErrSMTPSubmissionUncertain
	}
	if _, err := data.CloseWithResponse(); err != nil {
		return ErrSMTPSubmissionUncertain
	}
	// A successful DATA response is the durable SMTP acceptance boundary.
	// QUIT errors after that point must not turn an accepted message into a
	// retryable failure and cause a duplicate submission.
	_ = client.Quit()
	return nil
}

func (transport EmersionSMTP) connect(ctx context.Context, config ConnectionConfig) (*smtp.Client, error) {
	policy := transport.Policy
	if policy.SMTPPorts == nil {
		policy = DefaultEndpointPolicy()
	}
	raw, err := policy.DialContext(ctx, config.Host, config.Port, "smtp")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = raw.Close()
		}
	}()
	_ = raw.SetDeadline(commandDeadline(ctx, transport.CommandTimeout))
	var client *smtp.Client
	if config.Security == "tls" {
		tlsConnection := tls.Client(raw, tlsForHost(config.Host))
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return nil, ErrEndpointUnavailable
		}
		client = smtp.NewClient(tlsConnection)
	} else if config.Security == "starttls" {
		client, err = smtp.NewClientStartTLS(raw, tlsForHost(config.Host))
	} else {
		return nil, ErrEndpointRejected
	}
	if err != nil {
		return nil, ErrEndpointUnavailable
	}
	client.CommandTimeout = boundedCommandTimeout(transport.CommandTimeout)
	submission := transport.SubmissionTimeout
	if submission <= 0 || submission > time.Minute {
		submission = 30 * time.Second
	}
	client.SubmissionTimeout = submission
	keep = true
	return client, nil
}

func buildPlainMessage(from mail.Address, messageID string, input SendInput) ([]byte, error) {
	if strings.ContainsAny(messageID, "\r\n") || len(messageID) < 3 || len(messageID) > 998 ||
		strings.ContainsAny(input.Subject, "\r\n") || len([]rune(input.Subject)) > 2000 ||
		int64(len(input.PlainText)) > MaxBodyBytes {
		return nil, ErrMalformedMessage
	}
	header := messageMail.HeaderFromMap(map[string][]string{})
	header.SetDate(time.Now().UTC())
	header.SetAddressList("From", []*mail.Address{&from})
	header.SetAddressList("To", mailAddresses(input.Recipients.To))
	if len(input.Recipients.Cc) > 0 {
		header.SetAddressList("Cc", mailAddresses(input.Recipients.Cc))
	}
	header.SetSubject(input.Subject)
	header.SetMessageID(messageID)
	header.Set("Content-Type", "text/plain; charset=UTF-8")
	var buffer bytes.Buffer
	writer, err := messageMail.CreateSingleInlineWriter(&buffer, header)
	if err != nil {
		return nil, fmt.Errorf("create mail writer: %w", err)
	}
	if _, err := io.WriteString(writer, normalizePlainText(input.PlainText)); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("write mail body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish mail body: %w", err)
	}
	return buffer.Bytes(), nil
}

func validatedRecipients(input RecipientSet) ([]string, error) {
	total := len(input.To) + len(input.Cc) + len(input.Bcc)
	if len(input.To) == 0 || total > MaxRecipients {
		return nil, ErrMalformedMessage
	}
	result := make([]string, 0, total)
	seen := make(map[string]struct{}, total)
	for _, group := range [][]Address{input.To, input.Cc, input.Bcc} {
		for _, value := range group {
			parsed, err := exactAddress(value.Address)
			if err != nil || parsed.Name != "" || parsed.Address != strings.TrimSpace(value.Address) ||
				strings.ContainsAny(value.Name, "\r\n") || len([]rune(value.Name)) > 320 {
				return nil, ErrMalformedMessage
			}
			normalized := strings.ToLower(parsed.Address)
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = struct{}{}
				result = append(result, parsed.Address)
			}
		}
	}
	return result, nil
}

func mailAddresses(input []Address) []*mail.Address {
	result := make([]*mail.Address, 0, len(input))
	for _, value := range input {
		result = append(result, &mail.Address{Name: value.Name, Address: value.Address})
	}
	return result
}

func exactAddress(value string) (*mail.Address, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, ErrMalformedMessage
	}
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || !strings.Contains(parsed.Address, "@") || len(parsed.Address) > 254 {
		return nil, ErrMalformedMessage
	}
	return parsed, nil
}
