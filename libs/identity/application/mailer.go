package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

// Validation errors returned by Mailer implementations before any network I/O.
var (
	// ErrMailRecipientRequired is returned when Message.To is empty or whitespace.
	ErrMailRecipientRequired = errors.New("mail recipient is required")
	// ErrInvalidMailAddress is returned when an email address fails RFC 5322 parsing.
	ErrInvalidMailAddress = errors.New("invalid mail address")
	// ErrMailSubjectRequired is returned when Message.Subject is empty or whitespace.
	ErrMailSubjectRequired = errors.New("mail subject is required")
	// ErrMailBodyRequired is returned when both Message.Text and Message.HTML are empty.
	ErrMailBodyRequired = errors.New("mail body is required")
	// ErrMailHeaderInjection is returned when a header field or address contains CR, LF or another control character.
	ErrMailHeaderInjection = errors.New("mail header contains control characters")
)

// Message is an outbound email handed to a Mailer.
//
// Policy:
//   - To must be a single valid RFC 5322 email address without control characters.
//   - Subject must be non-empty (whitespace-only subjects are rejected) and free
//     of control characters.
//   - At least one of Text or HTML must be non-empty.
type Message struct {
	// To is the single recipient email address (e.g. "user@example.com" or "Name <user@example.com>").
	To string
	// Subject is the email subject line.
	Subject string
	// Text is the plain-text body content.
	Text string
	// HTML is the optional HTML body content.
	HTML string
}

// Mailer is the outbound port for sending transactional emails.
type Mailer interface {
	// Send validates and delivers one email message, honoring ctx cancellation and deadlines.
	Send(ctx context.Context, msg Message) error
}

// ValidateMessage checks msg against the Mailer port's validation and header-injection rules.
func ValidateMessage(msg Message) error {
	if hasControlChar(msg.To) {
		return fmt.Errorf("address: %w", ErrMailHeaderInjection)
	}
	if strings.TrimSpace(msg.To) == "" {
		return ErrMailRecipientRequired
	}
	if _, err := ValidateMailAddress(msg.To); err != nil {
		return err
	}
	if hasControlChar(msg.Subject) {
		return fmt.Errorf("subject: %w", ErrMailHeaderInjection)
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return ErrMailSubjectRequired
	}
	if strings.TrimSpace(msg.Text) == "" && strings.TrimSpace(msg.HTML) == "" {
		return ErrMailBodyRequired
	}
	return nil
}

// ValidateMailAddress parses a single RFC 5322 email address and rejects empty inputs
// or any ASCII/Unicode control characters (including CR and LF).
func ValidateMailAddress(raw string) (mail.Address, error) {
	if hasControlChar(raw) {
		return mail.Address{}, fmt.Errorf("address: %w", ErrMailHeaderInjection)
	}
	if strings.TrimSpace(raw) == "" {
		return mail.Address{}, ErrInvalidMailAddress
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return mail.Address{}, fmt.Errorf("%w: %w", ErrInvalidMailAddress, err)
	}
	if hasControlChar(addr.Address) || hasControlChar(addr.Name) {
		return mail.Address{}, fmt.Errorf("address: %w", ErrMailHeaderInjection)
	}
	local, domain, ok := strings.Cut(addr.Address, "@")
	if !ok || local == "" || domain == "" || strings.ContainsAny(addr.Address, " \t") {
		return mail.Address{}, ErrInvalidMailAddress
	}
	return *addr, nil
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
