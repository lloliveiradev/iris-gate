// Package mailer implements the identity application.Mailer port over SMTP
// using the Go standard library.
package mailer

import (
	"net/mail"
	"strings"
)

const redactedAddress = "<redacted>"

// MaskEmail returns a privacy-safe representation of an email address suitable
// for structured logs. Valid addresses are masked as "<first-rune>***@<domain>"
// (or "***@<domain>" for single-character local parts). Invalid or empty inputs
// return "<redacted>" so raw input is never logged.
func MaskEmail(raw string) string {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return redactedAddress
	}
	local, domain, ok := strings.Cut(addr.Address, "@")
	if !ok || local == "" || domain == "" {
		return redactedAddress
	}
	runes := []rune(local)
	if len(runes) <= 1 {
		return "***@" + domain
	}
	return string(runes[0]) + "***@" + domain
}

// scrubSecrets strips sensitive substrings from text before logging or returning
// errors that may contain server-echoed data.
func scrubSecrets(text string, secrets ...string) string {
	out := text
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, "[redacted]")
	}
	return out
}
