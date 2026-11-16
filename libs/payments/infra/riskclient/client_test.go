package riskclient

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/contracts/generated/go/fraudinternal"
	"github.com/iris-gate/iris-gate/libs/payments/application"
)

// slowServerDelay is far beyond Timeout: a slow server must be cut at Timeout.
const slowServerDelay = 2 * time.Second

func approveResponse() fraudinternal.RiskAssessmentResponse {
	return fraudinternal.RiskAssessmentResponse{
		AssessmentId: uuid.MustParse(testAssessmentID),
		Decision:     fraudinternal.APPROVE,
		Score:        testApproveScore,
		Reasons:      []fraudinternal.RiskReason{},
	}
}

func rejectResponse() fraudinternal.RiskAssessmentResponse {
	return fraudinternal.RiskAssessmentResponse{
		AssessmentId: uuid.MustParse(testAssessmentID),
		Decision:     fraudinternal.REJECT,
		Score:        testRejectScore,
		Reasons: []fraudinternal.RiskReason{
			{Code: fraudinternal.BLOCKEDCARD, Description: "card is blocked", Score: 1, Weight: 1},
		},
	}
}

func respondJSON(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func respondProblem(status int) http.HandlerFunc {
	return respondJSON(status, fraudinternal.Problem{Type: "about:blank", Title: http.StatusText(status), Status: status})
}

func respondRaw(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// waitForCancel blocks until the client gives up or slowServerDelay passes.
func waitForCancel(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(slowServerDelay):
	}
}

// assertRequest checks the wire request against the fraud-internal contract.
func assertRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost || r.URL.Path != assessPath {
		t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.Path, assessPath)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+testServiceToken {
		t.Errorf("Authorization header does not carry the service token")
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	var body fraudinternal.RiskAssessmentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Errorf("decode request body: %v", err)
		return
	}
	if body.PaymentIntentId.String() != testPaymentIntentID || body.MerchantId.String() != testMerchantID {
		t.Errorf("ids = %s / %s", body.PaymentIntentId, body.MerchantId)
	}
	if body.AmountCents != testAmountCents || body.Currency != testCurrency {
		t.Errorf("amount = %d %s", body.AmountCents, body.Currency)
	}
	if body.CardFingerprint != testCardFingerprint || body.CardBrand != testCardBrand {
		t.Errorf("card fields not forwarded")
	}
	if body.CustomerRef == nil || *body.CustomerRef != testCustomerRef {
		t.Errorf("customer_ref not forwarded")
	}
	if body.TestScenario != nil {
		t.Errorf("test_scenario = %q, want absent", *body.TestScenario)
	}
}

func TestClient_Assess(t *testing.T) {
	t.Parallel()

	unavailable := application.RiskUnavailableSnapshot()

	tests := []struct {
		name string
		// handler is nil for a server that is already closed.
		handler   http.HandlerFunc
		cancelCtx bool
		want      application.RiskSnapshot
		wantHits  int32
		// wantCause is empty when no fallback is expected.
		wantCause string
	}{
		{
			name:     "happy path approve",
			handler:  respondJSON(http.StatusOK, approveResponse()),
			want:     application.RiskSnapshot{AssessmentID: uuid.MustParse(testAssessmentID), Decision: application.RiskDecisionApprove, Score: testApproveScore, Reasons: []string{}},
			wantHits: 1,
		},
		{
			name:     "happy path reject keeps the reasons",
			handler:  respondJSON(http.StatusOK, rejectResponse()),
			want:     application.RiskSnapshot{AssessmentID: uuid.MustParse(testAssessmentID), Decision: application.RiskDecisionReject, Score: testRejectScore, Reasons: []string{string(fraudinternal.BLOCKEDCARD)}},
			wantHits: 1,
		},
		{
			name:      "slow server times out",
			handler:   func(_ http.ResponseWriter, r *http.Request) { waitForCancel(r) },
			want:      unavailable,
			wantHits:  1,
			wantCause: causeTimeout,
		},
		{
			name: "body cut by the deadline is a timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"assessment_id":`))
				w.(http.Flusher).Flush()
				waitForCancel(r)
			},
			want:      unavailable,
			wantHits:  1,
			wantCause: causeTimeout,
		},
		{
			name:      "500",
			handler:   respondProblem(http.StatusInternalServerError),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeStatus,
		},
		{
			name:      "401 missing or wrong service token",
			handler:   respondProblem(http.StatusUnauthorized),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeStatus,
		},
		{
			name:      "409 assessment conflict",
			handler:   respondProblem(http.StatusConflict),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeStatus,
		},
		{
			name: "redirect is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, assessPath+"/elsewhere", http.StatusTemporaryRedirect)
			},
			want:      unavailable,
			wantHits:  1,
			wantCause: causeStatus,
		},
		{
			name:      "garbage body",
			handler:   respondRaw(http.StatusOK, `<html>not json</html>`),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name:      "trailing data after the JSON document",
			handler:   respondRaw(http.StatusOK, `{"assessment_id":"`+testAssessmentID+`","decision":"APPROVE","score":0,"reasons":[],"latency_ms":1} {}`),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name: "unknown decision",
			handler: respondJSON(http.StatusOK, func() fraudinternal.RiskAssessmentResponse {
				r := approveResponse()
				r.Decision = "MAYBE"
				return r
			}()),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name: "missing assessment id",
			handler: respondJSON(http.StatusOK, func() fraudinternal.RiskAssessmentResponse {
				r := approveResponse()
				r.AssessmentId = uuid.Nil
				return r
			}()),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name: "score out of range",
			handler: respondJSON(http.StatusOK, func() fraudinternal.RiskAssessmentResponse {
				r := approveResponse()
				r.Score = maxScore + 1
				return r
			}()),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name: "unknown reason code",
			handler: respondJSON(http.StatusOK, func() fraudinternal.RiskAssessmentResponse {
				r := rejectResponse()
				r.Reasons[0].Code = "NOT_A_RULE"
				return r
			}()),
			want:      unavailable,
			wantHits:  1,
			wantCause: causeInvalidBody,
		},
		{
			name:      "network error",
			want:      unavailable,
			wantCause: causeNetwork,
		},
		{
			name:      "caller context already canceled",
			handler:   respondJSON(http.StatusOK, approveResponse()),
			cancelCtx: true,
			want:      unavailable,
			wantCause: causeNetwork,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				assertRequest(t, r)
				tc.handler(w, r)
			}))
			if tc.handler == nil {
				srv.Close()
			} else {
				t.Cleanup(srv.Close)
			}

			var logs bytes.Buffer
			client, err := New(Config{
				BaseURL:      srv.URL,
				ServiceToken: testServiceToken,
				Logger:       slog.New(slog.NewJSONHandler(&logs, nil)),
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelCtx {
				cancel()
			}

			start := time.Now()
			got := client.Assess(ctx, testRequest())
			elapsed := time.Since(start)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("snapshot = %+v, want %+v", got, tc.want)
			}
			if n := hits.Load(); n != tc.wantHits {
				t.Errorf("server hits = %d, want %d (no retries)", n, tc.wantHits)
			}
			if elapsed >= slowServerDelay/2 {
				t.Errorf("Assess took %v, want about %v at most", elapsed, Timeout)
			}
			assertLogs(t, logs.String(), tc.wantCause)
		})
	}
}

// assertLogs checks that a fallback logs its cause and the intent, and that no
// log line ever carries card data or the service token.
func assertLogs(t *testing.T, logs, wantCause string) {
	t.Helper()
	for _, secret := range secretValues() {
		if strings.Contains(logs, secret) {
			t.Errorf("logs leak %q: %s", secret, logs)
		}
	}
	if wantCause == "" {
		if logs != "" {
			t.Errorf("unexpected log on success: %s", logs)
		}
		return
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(logs), &entry); err != nil {
		t.Fatalf("want exactly one JSON log line, got %q: %v", logs, err)
	}
	if entry["level"] != slog.LevelWarn.String() || entry["cause"] != wantCause {
		t.Errorf("log level/cause = %v/%v, want %s/%s", entry["level"], entry["cause"], slog.LevelWarn, wantCause)
	}
	if entry["payment_intent_id"] != testPaymentIntentID {
		t.Errorf("log payment_intent_id = %v", entry["payment_intent_id"])
	}
}

func TestClient_Assess_SendsTestScenario(t *testing.T) {
	t.Parallel()

	scenario := make(chan *fraudinternal.RiskAssessmentRequestTestScenario, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body fraudinternal.RiskAssessmentRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		scenario <- body.TestScenario
		respondJSON(http.StatusOK, approveResponse())(w, r)
	}))
	t.Cleanup(srv.Close)

	client, err := New(Config{BaseURL: srv.URL, ServiceToken: testServiceToken, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := testRequest()
	req.TestScenario = string(fraudinternal.Review)
	client.Assess(context.Background(), req)

	if got := <-scenario; got == nil || *got != fraudinternal.Review {
		t.Errorf("test_scenario = %v, want %q", got, fraudinternal.Review)
	}
}

func TestNew_ValidatesConfig(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid", cfg: Config{BaseURL: testBaseURL, ServiceToken: testServiceToken, Logger: logger}},
		{name: "host without scheme", cfg: Config{BaseURL: testHostPort, ServiceToken: testServiceToken, Logger: logger}, wantErr: true},
		{name: "unsupported scheme", cfg: Config{BaseURL: "ftp://" + testHostPort, ServiceToken: testServiceToken, Logger: logger}, wantErr: true},
		{name: "missing service token", cfg: Config{BaseURL: testBaseURL, Logger: logger}, wantErr: true},
		{name: "missing logger", cfg: Config{BaseURL: testBaseURL, ServiceToken: testServiceToken}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("New err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
