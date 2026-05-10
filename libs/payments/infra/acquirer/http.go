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

// HTTPAcquirer é um adapter HTTP genérico — POSTa um payload JSON num
// endpoint configurável e parseia a resposta. Pronto pra apontar pra Stone
// sandbox (ou Cielo, Adyen, etc.) com ajuste fino dos campos quando
// integração real entrar.
//
// Contrato esperado pelo backend remoto:
//
//	POST {BaseURL}/authorize
//	Authorization: Bearer {Token}
//	Content-Type: application/json
//	{ "intent_id": "...", "amount_cents": ..., "currency": "BRL", "card_token": "..." }
//
//	→ 200 { "acquirer_ref": "...", "approved": true, "reason": "..." }
//	  ou non-2xx → ErrAcquirerHTTP{Status, Body}
type HTTPAcquirer struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewHTTP(baseURL, token string) *HTTPAcquirer {
	return &HTTPAcquirer{
		BaseURL: baseURL,
		Token:   token,
		Client:  &http.Client{Timeout: 10 * time.Second},
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

func (h *HTTPAcquirer) Authorize(ctx context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
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
		return application.AuthorizeOutput{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return application.AuthorizeOutput{}, fmt.Errorf("acquirer call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
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

type ErrAcquirerHTTP struct {
	Status int
	Body   string
}

func (e *ErrAcquirerHTTP) Error() string {
	return fmt.Sprintf("acquirer http %d: %s", e.Status, e.Body)
}
