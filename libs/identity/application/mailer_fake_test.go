package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iris-gate/iris-gate/libs/identity/application"
)

const (
	testMailRecipient = "alice@example.test"
	testMailSubject   = "Welcome to Iris Gate"
	testMailTextBody  = "Hello from Iris Gate."
	testMailHTMLBody  = "<p>Hello from Iris Gate.</p>"
)

func TestValidateMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msg     application.Message
		wantErr error
	}{
		{
			name: "valid text and html",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: testMailSubject,
				Text:    testMailTextBody,
				HTML:    testMailHTMLBody,
			},
		},
		{
			name: "valid text only",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
		},
		{
			name: "valid display name address",
			msg: application.Message{
				To:      "Alice <" + testMailRecipient + ">",
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
		},
		{
			name: "empty recipient",
			msg: application.Message{
				To:      "",
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailRecipientRequired,
		},
		{
			name: "whitespace recipient",
			msg: application.Message{
				To:      "   ",
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailRecipientRequired,
		},
		{
			name: "invalid recipient address",
			msg: application.Message{
				To:      "not-an-address",
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
			wantErr: application.ErrInvalidMailAddress,
		},
		{
			name: "crlf in recipient rejected",
			msg: application.Message{
				To:      testMailRecipient + "\r\nBcc: evil@example.test",
				Subject: testMailSubject,
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "lf in subject rejected",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: testMailSubject + "\nBcc: evil@example.test",
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "nul control char in subject rejected",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: testMailSubject + "\x00",
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "empty subject rejected",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: "",
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailSubjectRequired,
		},
		{
			name: "whitespace subject rejected",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: "   ",
				Text:    testMailTextBody,
			},
			wantErr: application.ErrMailSubjectRequired,
		},
		{
			name: "empty body rejected",
			msg: application.Message{
				To:      testMailRecipient,
				Subject: testMailSubject,
			},
			wantErr: application.ErrMailBodyRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := application.ValidateMessage(tt.msg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateMessage() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFakeMailer_RecordsAndFails(t *testing.T) {
	t.Parallel()

	fake := application.NewFakeMailer()
	msg := application.Message{
		To:      testMailRecipient,
		Subject: testMailSubject,
		Text:    testMailTextBody,
		HTML:    testMailHTMLBody,
	}

	if err := fake.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() unexpected error: %v", err)
	}
	got := fake.Messages()
	if len(got) != 1 || got[0] != msg {
		t.Fatalf("Messages() = %+v, want [%+v]", got, msg)
	}

	sentinel := errors.New("injected delivery failure")
	fake.FailWith(sentinel)
	err := fake.Send(context.Background(), msg)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Send() err = %v, want %v", err, sentinel)
	}
	if len(fake.Messages()) != 1 {
		t.Fatalf("Messages() count = %d after failed send, want 1", len(fake.Messages()))
	}

	fake.Reset()
	if len(fake.Messages()) != 0 {
		t.Fatalf("Messages() count = %d after Reset, want 0", len(fake.Messages()))
	}
	if err := fake.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() after Reset unexpected error: %v", err)
	}
}
