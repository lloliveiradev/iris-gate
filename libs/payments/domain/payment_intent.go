package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PaymentIntentStatus is the lifecycle state of a PaymentIntent.
type PaymentIntentStatus string

// Statuses of a PaymentIntent. Only pending, authorized, captured and failed
// have transitions today; refunded and canceled exist in the schema only.
const (
	PaymentIntentPending    PaymentIntentStatus = "pending"
	PaymentIntentAuthorized PaymentIntentStatus = "authorized"
	PaymentIntentCaptured   PaymentIntentStatus = "captured"
	PaymentIntentRefunded   PaymentIntentStatus = "refunded"
	PaymentIntentCanceled   PaymentIntentStatus = "canceled"
	PaymentIntentFailed     PaymentIntentStatus = "failed"
)

// FailureCode says why an intent ended in PaymentIntentFailed.
type FailureCode string

// FailureCardDeclined is an acquirer decline of the authorization.
const FailureCardDeclined FailureCode = "card_declined"

func (c FailureCode) known() bool {
	return c == FailureCardDeclined
}

// Card token length bounds and description limit accepted at creation.
const (
	minCardTokenLength   = 8
	maxCardTokenLength   = 256
	maxDescriptionLength = 200
)

// PaymentIntent is the payments aggregate.
type PaymentIntent struct {
	ID           uuid.UUID
	MerchantID   uuid.UUID
	Amount       Money
	Status       PaymentIntentStatus
	CardToken    string
	Description  string
	CreatedAt    time.Time
	AuthorizedAt *time.Time
	CapturedAt   *time.Time
	// FailureCode is set only when Status is PaymentIntentFailed.
	FailureCode FailureCode
	// Version is the optimistic-lock token: the persisted version this value
	// was loaded at. A repository updates the intent only while the stored
	// version still matches, and increments it on every update.
	Version int
}

// NewPaymentIntent validates its input and creates an intent in the initial
// `pending` status. The id and the clock are injected so the domain does no I/O.
func NewPaymentIntent(id, merchantID uuid.UUID, amount Money, cardToken, description string, now time.Time) (PaymentIntent, error) {
	if merchantID == uuid.Nil {
		return PaymentIntent{}, fmt.Errorf("%w: merchant_id required", ErrValidation)
	}
	if amount.IsZero() {
		return PaymentIntent{}, fmt.Errorf("%w: amount required", ErrValidation)
	}
	if l := len(cardToken); l < minCardTokenLength || l > maxCardTokenLength {
		return PaymentIntent{}, fmt.Errorf("%w: card_token length out of range (%d)", ErrValidation, l)
	}
	if len(description) > maxDescriptionLength {
		return PaymentIntent{}, fmt.Errorf("%w: description > %d chars", ErrValidation, maxDescriptionLength)
	}
	return PaymentIntent{
		ID:          id,
		MerchantID:  merchantID,
		Amount:      amount,
		Status:      PaymentIntentPending,
		CardToken:   cardToken,
		Description: description,
		CreatedAt:   now,
	}, nil
}

// AuthorizationNeeded tells the caller whether the acquirer must be asked:
// true for a pending intent, false for one already authorized (an idempotent
// retry). Any other status is ErrInvalidStatusTransition, so a captured or
// failed intent never reaches the acquirer.
func (p PaymentIntent) AuthorizationNeeded() (bool, error) {
	switch p.Status {
	case PaymentIntentPending:
		return true, nil
	case PaymentIntentAuthorized:
		return false, nil
	default:
		return false, fmt.Errorf("%w: cannot authorize from %s", ErrInvalidStatusTransition, p.Status)
	}
}

// MarkAuthorized moves pending → authorized. It is idempotent when the intent
// is already authorized, so callers need not tell a first call from a retry.
// Any other status yields ErrInvalidStatusTransition.
func (p *PaymentIntent) MarkAuthorized(at time.Time) error {
	switch p.Status {
	case PaymentIntentPending, PaymentIntentAuthorized:
		p.Status = PaymentIntentAuthorized
		p.AuthorizedAt = &at
		return nil
	default:
		return fmt.Errorf("%w: cannot authorize from %s", ErrInvalidStatusTransition, p.Status)
	}
}

// MarkCaptured moves authorized → captured. It is idempotent when the intent
// is already captured.
func (p *PaymentIntent) MarkCaptured(at time.Time) error {
	switch p.Status {
	case PaymentIntentAuthorized, PaymentIntentCaptured:
		p.Status = PaymentIntentCaptured
		p.CapturedAt = &at
		return nil
	default:
		return fmt.Errorf("%w: cannot capture from %s", ErrInvalidStatusTransition, p.Status)
	}
}

// MarkFailed records an acquirer decline: pending → failed with code, and
// returns the event to publish. On an intent that already failed it is a no-op
// that keeps the first code and returns no event. Any other status yields
// ErrInvalidStatusTransition; an unknown code yields ErrValidation.
func (p *PaymentIntent) MarkFailed(code FailureCode, eventID uuid.UUID, at time.Time) (*PaymentIntentFailedV1, error) {
	if !code.known() {
		return nil, fmt.Errorf("%w: unknown failure code %q", ErrValidation, code)
	}
	switch p.Status {
	case PaymentIntentFailed:
		return nil, nil
	case PaymentIntentPending:
		p.Status = PaymentIntentFailed
		p.FailureCode = code
		return &PaymentIntentFailedV1{
			EventID:         eventID,
			OccurredAt:      at,
			PaymentIntentID: p.ID,
			MerchantID:      p.MerchantID,
			FailureCode:     string(code),
			AmountCents:     p.Amount.AmountCents,
			Currency:        p.Amount.Currency,
		}, nil
	default:
		return nil, fmt.Errorf("%w: cannot fail from %s", ErrInvalidStatusTransition, p.Status)
	}
}
