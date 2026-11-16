package acquirer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iris-gate/iris-gate/libs/payments/application"
)

// HTTPAcquirer is a generic HTTP adapter: it POSTs a JSON payload to a
// configurable endpoint and parses the response. It can point at the Stone
// sandbox (or Cielo, Adyen, etc.) once the fields are tuned for a real
// integration.
//
// Contract expected from the remote backend:
//
//	POST {BaseURL}/authorize
//	Authorization: Bearer {Token}
//	Idempotency-Key: {AuthorizeInput.IdempotencyKey}
//	Content-Type: application/json
//	{ "intent_id": "...", "amount_cents": ..., "currency": "BRL", "card_token": "..." }
//
//	→ 200 { "acquirer_ref": "...", "approved": true, "reason": "..." }
//	  or non-2xx → ErrAcquirerHTTP{Status, Body}
//
// The backend must answer a repeated Idempotency-Key with the first result.
type HTTPAcquirer struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

// IdempotencyKeyHeader carries AuthorizeInput.IdempotencyKey to the backend.
const IdempotencyKeyHeader = "Idempotency-Key"

const defaultTimeout = 10 * time.Second

// NewHTTP returns an HTTPAcquirer for baseURL with a bearer token (optional).
func NewHTTP(baseURL, token string) *HTTPAcquirer {
	return &HTTPAcquirer{
		BaseURL: baseURL,
		Token:   token,
		Client:  &http.Client{Timeout: defaultTimeout},
	}
}

type httpAuthorizeReq struct {
	IntentID    string `json:"intent_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	CardToken   string `json:"card_token"`
}

type httpAuthorizeResp struct {
	AcquirerRef string `json:"acquirer_ref"`
	Approved    bool   `json:"approved"`
	Reason      string `json:"reason"`
}

// Authorize posts one authorization, keyed by in.IdempotencyKey.
func (h *HTTPAcquirer) Authorize(ctx context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	if in.IdempotencyKey == "" {
		return application.AuthorizeOutput{}, ErrIdempotencyKeyRequired
	}
	body, err := json.Marshal(httpAuthorizeReq{
		IntentID:    in.IntentID.String(),
		AmountCents: in.AmountCents,
		Currency:    in.Currency,
		CardToken:   in.CardToken,
	})
	if err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/authorize", bytes.NewReader(body))
	if err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(IdempotencyKeyHeader, in.IdempotencyKey)
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("acquirer call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return application.AuthorizeOutput{}, &ErrAcquirerHTTP{Status: resp.StatusCode, Body: string(respBody)}
	}

	var parsed httpAuthorizeResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("decode: %w", err)
	}
	return application.AuthorizeOutput{
		AcquirerRef: parsed.AcquirerRef,
		Approved:    parsed.Approved,
		Reason:      parsed.Reason,
	}, nil
}

// ErrAcquirerHTTP is a non-2xx answer from the acquirer backend.
type ErrAcquirerHTTP struct {
	Status int
	Body   string
}

// Error reports the status and the response body.
func (e *ErrAcquirerHTTP) Error() string {
	return fmt.Sprintf("acquirer http %d: %s", e.Status, e.Body)
}
