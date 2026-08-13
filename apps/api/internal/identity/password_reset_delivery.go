package identity

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

const passwordResetTemplateKey = "email.passwordReset"

type PasswordResetDelivery struct {
	Recipient   string
	Locale      string
	TemplateKey string
	Params      map[string]string
}

// PasswordResetSender is deliberately transport-neutral. Production can use a
// queued sender while local development can point SMTPPasswordResetSender at
// Mailpit without changing identity logic.
type PasswordResetSender interface {
	SendPasswordReset(ctx context.Context, delivery PasswordResetDelivery) error
}

type EmailRenderer interface {
	RenderEmail(templateKey, locale string, params map[string]string) (subject, textBody string, err error)
}

type SMTPPasswordResetSender struct {
	Address      string
	From         string
	Auth         smtp.Auth
	Renderer     EmailRenderer
	TLSConfig    *tls.Config
	RequireTLS   bool
	ImplicitTLS  bool
	WriteTimeout time.Duration
}

func (sender *SMTPPasswordResetSender) SendPasswordReset(ctx context.Context, delivery PasswordResetDelivery) error {
	if sender.Renderer == nil {
		return errors.New("password reset email renderer is required")
	}
	from, err := mail.ParseAddress(sender.From)
	if err != nil {
		return fmt.Errorf("parse SMTP from address: %w", err)
	}
	recipient, err := mail.ParseAddress(delivery.Recipient)
	if err != nil {
		return fmt.Errorf("parse reset recipient: %w", err)
	}
	subject, textBody, err := sender.Renderer.RenderEmail(delivery.TemplateKey, delivery.Locale, delivery.Params)
	if err != nil {
		return fmt.Errorf("render reset email: %w", err)
	}
	if containsHeaderBreak(subject) || containsHeaderBreak(from.String()) || containsHeaderBreak(recipient.String()) {
		return errors.New("email header contains a line break")
	}

	host, _, err := net.SplitHostPort(sender.Address)
	if err != nil {
		return fmt.Errorf("SMTP address must include host and port: %w", err)
	}
	timeout := sender.WriteTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	var connection net.Conn
	if sender.ImplicitTLS {
		tlsConfig := sender.tlsConfig(host)
		connection, err = tls.DialWithDialer(&dialer, "tcp", sender.Address, tlsConfig)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", sender.Address)
	}
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("initialize SMTP: %w", err)
	}
	defer client.Close()
	if !sender.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(sender.tlsConfig(host)); err != nil {
				return fmt.Errorf("start SMTP TLS: %w", err)
			}
		} else if sender.RequireTLS {
			return errors.New("SMTP server does not advertise STARTTLS")
		}
	}
	if sender.Auth != nil {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("SMTP server does not advertise AUTH")
		}
		if err := client.Auth(sender.Auth); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	data, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP body: %w", err)
	}
	if err := writePlainTextEmail(data, from.String(), recipient.String(), subject, textBody); err != nil {
		_ = data.Close()
		return fmt.Errorf("write SMTP body: %w", err)
	}
	if err := data.Close(); err != nil {
		return fmt.Errorf("finish SMTP body: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP: %w", err)
	}
	return nil
}

func (sender *SMTPPasswordResetSender) tlsConfig(host string) *tls.Config {
	if sender.TLSConfig != nil {
		clone := sender.TLSConfig.Clone()
		if clone.ServerName == "" {
			clone.ServerName = host
		}
		clone.MinVersion = max(clone.MinVersion, tls.VersionTLS12)
		return clone
	}
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

func writePlainTextEmail(writer io.Writer, from, to, subject, textBody string) error {
	if containsHeaderBreak(from) || containsHeaderBreak(to) || containsHeaderBreak(subject) {
		return errors.New("email header contains a line break")
	}
	buffer := bufio.NewWriter(writer)
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	for _, header := range headers {
		if containsHeaderBreak(header) {
			return errors.New("email header contains a line break")
		}
		if _, err := buffer.WriteString(header + "\r\n"); err != nil {
			return err
		}
	}
	if _, err := buffer.WriteString("\r\n"); err != nil {
		return err
	}
	normalizedBody := strings.ReplaceAll(strings.ReplaceAll(textBody, "\r\n", "\n"), "\r", "\n")
	if _, err := buffer.WriteString(strings.ReplaceAll(normalizedBody, "\n", "\r\n")); err != nil {
		return err
	}
	return buffer.Flush()
}

func containsHeaderBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}
