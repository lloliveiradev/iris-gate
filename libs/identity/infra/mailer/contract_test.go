package mailer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/infra/mailer"
)

type contractHarness struct {
	mailer      application.Mailer
	countSent   func() int
	forceFailed func()
	clearFailed func()
}

// TestMailerContract runs the shared Liskov contract suite against both the
// in-memory FakeMailer and the stdlib SMTPMailer (using an in-process SMTP server).
func TestMailerContract(t *testing.T) {
	t.Parallel()

	adapters := []struct {
		name  string
		setup func(t *testing.T) contractHarness
	}{
		{
			name: "FakeMailer",
			setup: func(t *testing.T) contractHarness {
				t.Helper()
				fake := application.NewFakeMailer()
				return contractHarness{
					mailer:      fake,
					countSent:   func() int { return len(fake.Messages()) },
					forceFailed: func() { fake.FailWith(errors.New("simulated transport failure")) },
					clearFailed: func() { fake.FailWith(nil) },
				}
			},
		},
		{
			name: "SMTPMailer",
			setup: func(t *testing.T) contractHarness {
				t.Helper()
				srv := startFakeSMTPServer(t)
				adapter, err := mailer.NewSMTP(mailer.Config{
					Host: srv.host,
					Port: srv.port,
					From: testFromAddress,
				})
				if err != nil {
					t.Fatalf("NewSMTP: %v", err)
				}
				return contractHarness{
					mailer: adapter,
					countSent: func() int {
						_, _, msgs := srv.snapshot()
						return len(msgs)
					},
					forceFailed: func() {
						srv.mu.Lock()
						srv.rejectRcptMsg = "5.1.1 recipient rejected"
						srv.mu.Unlock()
					},
					clearFailed: func() {
						srv.mu.Lock()
						srv.rejectRcptMsg = ""
						srv.mu.Unlock()
					},
				}
			},
		},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			t.Parallel()
			runMailerContract(t, adapter.setup)
		})
	}
}

func runMailerContract(t *testing.T, setup func(t *testing.T) contractHarness) {
	t.Helper()

	t.Run("records valid text and html message", func(t *testing.T) {
		h := setup(t)
		if err := h.mailer.Send(context.Background(), validMessage()); err != nil {
			t.Fatalf("Send() unexpected error: %v", err)
		}
		if got := h.countSent(); got != 1 {
			t.Fatalf("sent count = %d, want 1", got)
		}
	})

	t.Run("records valid text-only message", func(t *testing.T) {
		h := setup(t)
		msg := application.Message{
			To:      testRecipientAddress,
			Subject: testSubject,
			Text:    testTextBody,
		}
		if err := h.mailer.Send(context.Background(), msg); err != nil {
			t.Fatalf("Send() unexpected error: %v", err)
		}
		if got := h.countSent(); got != 1 {
			t.Fatalf("sent count = %d, want 1", got)
		}
	})

	invalidCases := []struct {
		name    string
		msg     application.Message
		wantErr error
	}{
		{
			name: "empty recipient",
			msg: application.Message{
				To:      "",
				Subject: testSubject,
				Text:    testTextBody,
			},
			wantErr: application.ErrMailRecipientRequired,
		},
		{
			name: "invalid recipient",
			msg: application.Message{
				To:      "not-a-valid-email",
				Subject: testSubject,
				Text:    testTextBody,
			},
			wantErr: application.ErrInvalidMailAddress,
		},
		{
			name: "header injection in recipient",
			msg: application.Message{
				To:      testRecipientAddress + "\r\nBcc: " + testInjectedAddress,
				Subject: testSubject,
				Text:    testTextBody,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "header injection in subject",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: testSubject + "\r\nBcc: " + testInjectedAddress,
				Text:    testTextBody,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "empty subject",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: "",
				Text:    testTextBody,
			},
			wantErr: application.ErrMailSubjectRequired,
		},
		{
			name: "empty body",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: testSubject,
			},
			wantErr: application.ErrMailBodyRequired,
		},
	}

	for _, tc := range invalidCases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			h := setup(t)
			err := h.mailer.Send(context.Background(), tc.msg)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Send() err = %v, want %v", err, tc.wantErr)
			}
			if got := h.countSent(); got != 0 {
				t.Fatalf("sent count = %d, want 0", got)
			}
		})
	}

	t.Run("honors cancelled context", func(t *testing.T) {
		h := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := h.mailer.Send(ctx, validMessage())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Send() err = %v, want context.Canceled", err)
		}
		if got := h.countSent(); got != 0 {
			t.Fatalf("sent count = %d, want 0", got)
		}
	})

	t.Run("returns error on delivery failure without recording", func(t *testing.T) {
		h := setup(t)
		h.forceFailed()
		defer h.clearFailed()

		err := h.mailer.Send(context.Background(), validMessage())
		if err == nil {
			t.Fatal("Send() err = nil, want non-nil error")
		}
		if got := h.countSent(); got != 0 {
			t.Fatalf("sent count = %d, want 0", got)
		}
	})
}
