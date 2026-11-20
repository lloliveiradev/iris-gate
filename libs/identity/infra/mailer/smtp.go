package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/iris-gate/iris-gate/libs/identity/application"
)

// DefaultTimeout bounds one SMTP delivery (dial, handshake, auth, and transfer)
// when the caller's context carries no shorter deadline.
const DefaultTimeout = 10 * time.Second

const (
	minPort = 1
	maxPort = 65535
)

// TLSMode controls whether the SMTP adapter negotiates STARTTLS.
type TLSMode string

const (
	// TLSNone uses plaintext SMTP (local Mailpit).
	TLSNone TLSMode = "none"
	// TLSStartTLS upgrades the connection with STARTTLS before AUTH and transfer.
	TLSStartTLS TLSMode = "starttls"
)

// Adapter errors.
var (
	// ErrInvalidConfig is returned by NewSMTP when configuration values are invalid.
	ErrInvalidConfig = errors.New("invalid mailer config")
	// ErrAuthFailed is returned when SMTP PLAIN authentication fails.
	// Credentials are never echoed in the returned error.
	ErrAuthFailed = errors.New("smtp authentication failed")
)

// Config holds the settings for SMTPMailer.
type Config struct {
	Host     string
	Port     int
	From     string
	Username string
	// Password is optional and must never be logged or echoed in errors.
	Password string
	// TLSMode selects plaintext ("none", default) or "starttls".
	TLSMode TLSMode
	// Timeout caps one Send call when ctx has no earlier deadline. Defaults to DefaultTimeout.
	Timeout time.Duration
	// Logger receives structured delivery events with masked recipients. Optional.
	Logger *slog.Logger
}

// SMTPMailer implements application.Mailer over stdlib net/smtp.
type SMTPMailer struct {
	host     string
	port     int
	fromAddr mail.Address
	username string
	password string
	tlsMode  TLSMode
	timeout  time.Duration
	logger   *slog.Logger
}

var _ application.Mailer = (*SMTPMailer)(nil)

// NewSMTP validates cfg and builds an SMTPMailer without performing network I/O.
func NewSMTP(cfg Config) (*SMTPMailer, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" || containsControl(cfg.Host) {
		return nil, fmt.Errorf("%w: host must be non-empty without control characters", ErrInvalidConfig)
	}
	if cfg.Port < minPort || cfg.Port > maxPort {
		return nil, fmt.Errorf("%w: port must be between %d and %d", ErrInvalidConfig, minPort, maxPort)
	}
	fromAddr, err := application.ValidateMailAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("%w: from: %w", ErrInvalidConfig, err)
	}
	if cfg.Password != "" && cfg.Username == "" {
		return nil, fmt.Errorf("%w: password requires username", ErrInvalidConfig)
	}
	if containsControl(cfg.Username) || containsControl(cfg.Password) {
		return nil, fmt.Errorf("%w: credentials must not contain control characters", ErrInvalidConfig)
	}

	tlsMode := cfg.TLSMode
	if tlsMode == "" {
		tlsMode = TLSNone
	}
	if tlsMode != TLSNone && tlsMode != TLSStartTLS {
		return nil, fmt.Errorf("%w: unsupported TLS mode %q", ErrInvalidConfig, tlsMode)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &SMTPMailer{
		host:     host,
		port:     cfg.Port,
		fromAddr: fromAddr,
		username: cfg.Username,
		password: cfg.Password,
		tlsMode:  tlsMode,
		timeout:  timeout,
		logger:   logger,
	}, nil
}

// Send validates msg, builds a multipart/alternative email, and delivers it over SMTP.
// Context cancellation and deadlines stop both the TCP dial and the SMTP session.
// Recipients are never logged in clear.
func (m *SMTPMailer) Send(ctx context.Context, msg application.Message) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := application.ValidateMessage(msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	toAddr, err := application.ValidateMailAddress(msg.To)
	if err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	rawMessage, err := buildMIMEMessage(m.fromAddr, toAddr, msg, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("send mail: build message: %w", err)
	}

	sendCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	stage, err := m.deliver(sendCtx, toAddr.Address, rawMessage)
	if err != nil {
		switch {
		case sendCtx.Err() != nil:
			err = sendCtx.Err()
		case errors.Is(err, os.ErrDeadlineExceeded):
			// conn.SetDeadline can fire slightly before sendCtx's timer callback updates sendCtx.Err().
			err = context.DeadlineExceeded
		}
		m.logFailure(ctx, msg.To, stage, err, toAddr.Address)
		return fmt.Errorf("send mail: %s: %w", stage, err)
	}

	m.logger.LogAttrs(ctx, slog.LevelInfo, "mailer.sent",
		slog.String("recipient", MaskEmail(msg.To)),
	)
	return nil
}

func (m *SMTPMailer) deliver(ctx context.Context, recipient string, rawMessage []byte) (string, error) {
	addr := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "dial", err
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return "deadline", err
		}
	}
	stopCancel := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stopCancel()

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return "connect", err
	}
	closed := false
	defer func() {
		if !closed {
			_ = client.Close()
		}
	}()

	if m.tlsMode == TLSStartTLS {
		if err := client.StartTLS(&tls.Config{
			ServerName: m.host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return "starttls", err
		}
	}

	if m.username != "" {
		auth := smtp.PlainAuth("", m.username, m.password, m.host)
		if err := client.Auth(auth); err != nil {
			return "auth", sanitizeAuthError(err)
		}
	}

	if err := client.Mail(m.fromAddr.Address); err != nil {
		return "mail_from", err
	}
	if err := client.Rcpt(recipient); err != nil {
		return "rcpt_to", err
	}

	writer, err := client.Data()
	if err != nil {
		return "data", err
	}
	if _, err := writer.Write(rawMessage); err != nil {
		return "data_write", err
	}
	if err := writer.Close(); err != nil {
		return "data_close", err
	}

	if err := client.Quit(); err != nil {
		return "quit", err
	}
	closed = true
	return "", nil
}

func (m *SMTPMailer) logFailure(ctx context.Context, rawTo, stage string, err error, parsedTo string) {
	attrs := []slog.Attr{
		slog.String("recipient", MaskEmail(rawTo)),
		slog.String("stage", stage),
		slog.String("cause", scrubSecrets(
			err.Error(),
			rawTo,
			parsedTo,
			strings.ToLower(parsedTo),
			strings.ToUpper(parsedTo),
			m.password,
		)),
	}
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		attrs = append(attrs, slog.Int("smtp_code", protoErr.Code))
	}
	m.logger.LogAttrs(ctx, slog.LevelWarn, "mailer.send_failed", attrs...)
}

// sanitizeAuthError wraps ErrAuthFailed without returning server response text
// that could echo the password or encoded credentials.
func sanitizeAuthError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		return fmt.Errorf("smtp code %d: %w", protoErr.Code, ErrAuthFailed)
	}
	return ErrAuthFailed
}

func containsControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
