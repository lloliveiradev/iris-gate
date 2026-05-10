package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type MerchantStatus string

const (
	MerchantPending   MerchantStatus = "pending"
	MerchantActive    MerchantStatus = "active"
	MerchantSuspended MerchantStatus = "suspended"
)

type Merchant struct {
	ID           uuid.UUID
	LegalName    string
	Document     string
	ContactEmail string
	Status       MerchantStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var (
	documentRE = regexp.MustCompile(`^[0-9]{14}$`)
	// Email regex deliberadamente permissivo — validação pesada fica para a borda
	// (formato visto pelo usuário) ou para um adapter dedicado.
	emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// NewMerchant valida os campos e devolve um Merchant pronto.
// Não toca em I/O; ID e clock são injetados pelo chamador (use case).
func NewMerchant(id uuid.UUID, legalName, document, email string, now time.Time) (Merchant, error) {
	legalName = strings.TrimSpace(legalName)
	if l := len(legalName); l < 1 || l > 200 {
		return Merchant{}, fmt.Errorf("%w: legal_name length %d", ErrValidation, l)
	}
	if !documentRE.MatchString(document) {
		return Merchant{}, fmt.Errorf("%w: document must be 14 digits", ErrValidation)
	}
	if !emailRE.MatchString(email) {
		return Merchant{}, fmt.Errorf("%w: contact_email", ErrValidation)
	}
	return Merchant{
		ID:           id,
		LegalName:    legalName,
		Document:     document,
		ContactEmail: email,
		Status:       MerchantPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
