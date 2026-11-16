// Package riskclient is the HTTP adapter of the application.RiskAssessor port.
// It calls the anti-fraud scoring API (libs/contracts/http/fraud-internal.openapi.yaml)
// and fails closed: any failure becomes REVIEW with reason RISK_UNAVAILABLE.
package riskclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/contracts/generated/go/fraudinternal"
	"github.com/iris-gate/iris-gate/libs/payments/application"
)

// Timeout is the total budget for one assessment: connect, request, response
// headers and body. There are no retries within or after it.
const Timeout = 300 * time.Millisecond

const (
	assessPath       = "/internal/v1/risk-assessments"
	maxResponseBytes = 64 << 10
	minScore         = 0
	maxScore         = 100
)

// Causes logged when the client falls back to RISK_UNAVAILABLE.
const (
	causeEncodeRequest = "encode_request"
	causeTimeout       = "timeout"
	causeNetwork       = "network_error"
	causeStatus        = "non_2xx_status"
	causeInvalidBody   = "invalid_body"
)

// decisions maps the contract's decisions to the application's. A value
// missing here is an invalid response.
var decisions = map[fraudinternal.RiskDecision]application.RiskDecision{
	fraudinternal.APPROVE: application.RiskDecisionApprove,
	fraudinternal.REVIEW:  application.RiskDecisionReview,
	fraudinternal.REJECT:  application.RiskDecisionReject,
}

// Config holds the client settings.
type Config struct {
	// BaseURL is the absolute http(s) URL of the anti-fraud internal API.
	BaseURL string
	// ServiceToken is sent as a bearer token. It is never logged.
	ServiceToken string
	// Logger receives the cause of every fail-closed fallback.
	Logger *slog.Logger
}

// Client implements application.RiskAssessor over HTTP.
type Client struct {
	endpoint     string
	serviceToken string
	http         *http.Client
	logger       *slog.Logger
}

var _ application.RiskAssessor = (*Client)(nil)

// New validates cfg and builds a Client.
func New(cfg Config) (*Client, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, errors.New("riskclient: base URL must be an absolute http(s) URL")
	}
	if cfg.ServiceToken == "" {
		return nil, errors.New("riskclient: service token is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("riskclient: logger is required")
	}
	return &Client{
		endpoint:     base.JoinPath(assessPath).String(),
		serviceToken: cfg.ServiceToken,
		http: &http.Client{
			// A redirect is a non-2xx answer: fail closed instead of following it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		logger: cfg.Logger,
	}, nil
}

// Assess calls the scoring API once within Timeout. Any failure returns
// application.RiskUnavailableSnapshot and logs the cause without card data.
func (c *Client) Assess(ctx context.Context, req application.RiskAssessmentRequest) application.RiskSnapshot {
	snapshot, cause, status, err := c.assess(ctx, req)
	if err != nil {
		attrs := []slog.Attr{
			slog.String("cause", cause),
			slog.String("payment_intent_id", req.PaymentIntentID.String()),
			slog.String("merchant_id", req.MerchantID.String()),
			slog.String("err", err.Error()),
		}
		if status != 0 {
			attrs = append(attrs, slog.Int("status", status))
		}
		c.logger.LogAttrs(ctx, slog.LevelWarn, "riskclient.unavailable", attrs...)
		return application.RiskUnavailableSnapshot()
	}
	return snapshot
}

// assess returns the snapshot, or the cause, HTTP status (0 when none) and
// error of the failure.
func (c *Client) assess(ctx context.Context, req application.RiskAssessmentRequest) (application.RiskSnapshot, string, int, error) {
	body, err := json.Marshal(toRequestDTO(req))
	if err != nil {
		return application.RiskSnapshot{}, causeEncodeRequest, 0, fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return application.RiskSnapshot{}, causeEncodeRequest, 0, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.serviceToken)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return application.RiskSnapshot{}, failureCause(ctx, causeNetwork), 0, fmt.Errorf("call scoring API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return application.RiskSnapshot{}, causeStatus, resp.StatusCode, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	dto, err := decodeResponse(resp.Body)
	if err != nil {
		return application.RiskSnapshot{}, failureCause(ctx, causeInvalidBody), resp.StatusCode, err
	}
	snapshot, err := toSnapshot(dto)
	if err != nil {
		return application.RiskSnapshot{}, causeInvalidBody, resp.StatusCode, err
	}
	return snapshot, "", 0, nil
}

func toRequestDTO(req application.RiskAssessmentRequest) fraudinternal.RiskAssessmentRequest {
	dto := fraudinternal.RiskAssessmentRequest{
		PaymentIntentId: req.PaymentIntentID,
		MerchantId:      req.MerchantID,
		AmountCents:     req.Amount.AmountCents,
		Currency:        req.Amount.Currency,
		CardFingerprint: req.CardFingerprint,
		CardBrand:       req.CardBrand,
	}
	if req.CustomerRef != "" {
		customerRef := req.CustomerRef
		dto.CustomerRef = &customerRef
	}
	if req.TestScenario != "" {
		scenario := fraudinternal.RiskAssessmentRequestTestScenario(req.TestScenario)
		dto.TestScenario = &scenario
	}
	return dto
}

// decodeResponse reads exactly one JSON document from a bounded body.
func decodeResponse(body io.Reader) (fraudinternal.RiskAssessmentResponse, error) {
	var dto fraudinternal.RiskAssessmentResponse
	dec := json.NewDecoder(io.LimitReader(body, maxResponseBytes))
	if err := dec.Decode(&dto); err != nil {
		return dto, fmt.Errorf("decode response: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return dto, errors.New("decode response: unexpected data after the JSON document")
	}
	return dto, nil
}

// toSnapshot rejects any response that does not describe a complete,
// in-contract assessment.
func toSnapshot(dto fraudinternal.RiskAssessmentResponse) (application.RiskSnapshot, error) {
	if dto.AssessmentId == uuid.Nil {
		return application.RiskSnapshot{}, errors.New("invalid response: missing assessment_id")
	}
	decision, ok := decisions[dto.Decision]
	if !ok {
		return application.RiskSnapshot{}, fmt.Errorf("invalid response: unknown decision %q", dto.Decision)
	}
	score := float64(dto.Score)
	if math.IsNaN(score) || score < minScore || score > maxScore {
		return application.RiskSnapshot{}, fmt.Errorf("invalid response: score %v out of range", score)
	}
	reasons := make([]string, 0, len(dto.Reasons))
	for _, reason := range dto.Reasons {
		if !reason.Code.Valid() {
			return application.RiskSnapshot{}, fmt.Errorf("invalid response: unknown reason code %q", reason.Code)
		}
		reasons = append(reasons, string(reason.Code))
	}
	return application.RiskSnapshot{
		AssessmentID: dto.AssessmentId,
		Decision:     decision,
		Score:        score,
		Reasons:      reasons,
	}, nil
}

// failureCause reports a timeout when the call's deadline expired, so a body
// cut by the deadline is not mistaken for a malformed one.
func failureCause(ctx context.Context, otherwise string) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return causeTimeout
	}
	return otherwise
}
