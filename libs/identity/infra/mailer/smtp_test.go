package mailer_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/infra/mailer"
)

func TestNewSMTP_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     mailer.Config
		wantErr error
	}{
		{
			name: "valid minimal config",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 11025,
				From: testFromAddress,
			},
		},
		{
			name: "valid display-name from and starttls",
			cfg: mailer.Config{
				Host:     testLocalHost,
				Port:     587,
				From:     testFromDisplay,
				Username: testUsername,
				Password: testPassword,
				TLSMode:  mailer.TLSStartTLS,
			},
		},
		{
			name: "empty host rejected",
			cfg: mailer.Config{
				Host: "   ",
				Port: 11025,
				From: testFromAddress,
			},
			wantErr: mailer.ErrInvalidConfig,
		},
		{
			name: "port zero rejected",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 0,
				From: testFromAddress,
			},
			wantErr: mailer.ErrInvalidConfig,
		},
		{
			name: "port above 65535 rejected",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 70000,
				From: testFromAddress,
			},
			wantErr: mailer.ErrInvalidConfig,
		},
		{
			name: "empty from rejected",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 11025,
				From: "",
			},
			wantErr: application.ErrInvalidMailAddress,
		},
		{
			name: "invalid from rejected",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 11025,
				From: "not-an-address",
			},
			wantErr: application.ErrInvalidMailAddress,
		},
		{
			name: "crlf in from rejected before network IO",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 11025,
				From: testFromAddress + "\r\nBcc: " + testInjectedAddress,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "lf in from rejected",
			cfg: mailer.Config{
				Host: testLocalHost,
				Port: 11025,
				From: testFromAddress + "\nBcc: " + testInjectedAddress,
			},
			wantErr: application.ErrMailHeaderInjection,
		},
		{
			name: "password without username rejected",
			cfg: mailer.Config{
				Host:     testLocalHost,
				Port:     11025,
				From:     testFromAddress,
				Password: testPassword,
			},
			wantErr: mailer.ErrInvalidConfig,
		},
		{
			name: "unsupported TLS mode rejected",
			cfg: mailer.Config{
				Host:    testLocalHost,
				Port:    11025,
				From:    testFromAddress,
				TLSMode: mailer.TLSMode("bogus"),
			},
			wantErr: mailer.ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := mailer.NewSMTP(tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSMTP() err = %v, want %v", err, tt.wantErr)
			}
			if err != nil && tt.cfg.Password != "" && strings.Contains(err.Error(), tt.cfg.Password) {
				t.Fatalf("NewSMTP() error leaked password: %v", err)
			}
		})
	}
}

func TestSMTPMailer_HeaderInjectionRejectedBeforeNetworkIO(t *testing.T) {
	t.Parallel()

	srv := startFakeSMTPServer(t)
	adapter, err := mailer.NewSMTP(mailer.Config{
		Host: srv.host,
		Port: srv.port,
		From: testFromAddress,
	})
	if err != nil {
		t.Fatalf("NewSMTP: %v", err)
	}

	cases := []struct {
		name string
		msg  application.Message
	}{
		{
			name: "CR in subject",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: testSubject + "\rBcc: " + testInjectedAddress,
				Text:    testTextBody,
			},
		},
		{
			name: "LF in subject",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: testSubject + "\nBcc: " + testInjectedAddress,
				Text:    testTextBody,
			},
		},
		{
			name: "CRLF in subject",
			msg: application.Message{
				To:      testRecipientAddress,
				Subject: testSubject + "\r\nBcc: " + testInjectedAddress,
				Text:    testTextBody,
			},
		},
		{
			name: "CRLF in recipient",
			msg: application.Message{
				To:      testRecipientAddress + "\r\nBcc: " + testInjectedAddress,
				Subject: testSubject,
				Text:    testTextBody,
			},
		},
		{
			name: "tab control char in recipient",
			msg: application.Message{
				To:      testRecipientAddress + "\t",
				Subject: testSubject,
				Text:    testTextBody,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := adapter.Send(context.Background(), tc.msg)
			if !errors.Is(err, application.ErrMailHeaderInjection) {
				t.Fatalf("Send() err = %v, want ErrMailHeaderInjection", err)
			}
		})
	}

	conns, _, msgs := srv.snapshot()
	if conns != 0 || len(msgs) != 0 {
		t.Fatalf("server saw %d connections and %d messages, want 0", conns, len(msgs))
	}
}

func TestSMTPMailer_MultipartAlternativeAndBaseLayout(t *testing.T) {
	t.Parallel()

	srv := startFakeSMTPServer(t)
	adapter, err := mailer.NewSMTP(mailer.Config{
		Host: srv.host,
		Port: srv.port,
		From: testFromDisplay,
	})
	if err != nil {
		t.Fatalf("NewSMTP: %v", err)
	}

	t.Run("plain text only renders both text and escaped html base layout", func(t *testing.T) {
		srv.reset()
		msg := application.Message{
			To:      testRecipientDisplay,
			Subject: testUTF8Subject,
			Text:    "Hello <script>alert(1)</script>\n\nSecond paragraph.",
		}
		if err := adapter.Send(context.Background(), msg); err != nil {
			t.Fatalf("Send() err = %v", err)
		}

		_, _, msgs := srv.snapshot()
		if len(msgs) != 1 {
			t.Fatalf("captured messages = %d, want 1", len(msgs))
		}
		if msgs[0].MailFrom != testFromAddress {
			t.Errorf("MAIL FROM = %q, want %q", msgs[0].MailFrom, testFromAddress)
		}
		if len(msgs[0].RcptTo) != 1 || msgs[0].RcptTo[0] != testRecipientAddress {
			t.Errorf("RCPT TO = %v, want [%q]", msgs[0].RcptTo, testRecipientAddress)
		}

		textPart, htmlPart, headers := parseMultipartMessage(t, msgs[0].RawData)
		if headers.Get("MIME-Version") != "1.0" {
			t.Errorf("MIME-Version = %q, want 1.0", headers.Get("MIME-Version"))
		}
		if !strings.Contains(textPart, "Iris Gate") || !strings.Contains(textPart, "Second paragraph.") {
			t.Errorf("text part missing base layout or body: %q", textPart)
		}
		if !strings.Contains(htmlPart, `<!DOCTYPE html>`) || !strings.Contains(htmlPart, `<html lang="en">`) {
			t.Errorf("html part missing base layout: %q", htmlPart)
		}
		if strings.Contains(htmlPart, "<script>") {
			t.Errorf("html part did not escape script tag: %q", htmlPart)
		}
		if !strings.Contains(htmlPart, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("html part missing escaped body: %q", htmlPart)
		}
	})

	t.Run("full html document passes through without double wrapping", func(t *testing.T) {
		srv.reset()
		msg := application.Message{
			To:      testRecipientAddress,
			Subject: testSubject,
			Text:    testTextBody,
			HTML:    testFullHTMLDoc,
		}
		if err := adapter.Send(context.Background(), msg); err != nil {
			t.Fatalf("Send() err = %v", err)
		}
		_, _, msgs := srv.snapshot()
		_, htmlPart, _ := parseMultipartMessage(t, msgs[0].RawData)
		if htmlPart != testFullHTMLDoc {
			t.Errorf("html part = %q, want %q", htmlPart, testFullHTMLDoc)
		}
	})
}

func TestSMTPMailer_ServerDownAndContextTimeout(t *testing.T) {
	t.Parallel()

	t.Run("connection refused returns wrapped error without panic or hang", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen ephemeral port: %v", err)
		}
		closedPort := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:    testLocalHost,
			Port:    closedPort,
			From:    testFromAddress,
			Timeout: 2 * time.Second,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		err = adapter.Send(context.Background(), validMessage())
		if err == nil {
			t.Fatal("Send() err = nil, want connection refused error")
		}
		if !strings.Contains(err.Error(), "send mail:") {
			t.Errorf("Send() err = %q, want wrapped 'send mail:' prefix", err.Error())
		}
	})

	t.Run("context deadline stops stalled SMTP server", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		unblock := make(chan struct{})
		t.Cleanup(func() { close(unblock) })
		srv.mu.Lock()
		srv.stallOnConnect = unblock
		srv.mu.Unlock()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:    srv.host,
			Port:    srv.port,
			From:    testFromAddress,
			Timeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = adapter.Send(ctx, validMessage())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Send() err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("context cancel stops stalled DATA phase", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		unblock := make(chan struct{})
		t.Cleanup(func() { close(unblock) })
		srv.mu.Lock()
		srv.stallOnData = unblock
		srv.mu.Unlock()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:    srv.host,
			Port:    srv.port,
			From:    testFromAddress,
			Timeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(80 * time.Millisecond)
			cancel()
		}()

		err = adapter.Send(ctx, validMessage())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Send() err = %v, want context.Canceled", err)
		}
	})
}

func TestSMTPMailer_ServerRejections(t *testing.T) {
	t.Parallel()

	t.Run("recipient rejected returns wrapped error", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		srv.mu.Lock()
		srv.rejectRcptMsg = "5.1.1 user unknown"
		srv.mu.Unlock()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host: srv.host,
			Port: srv.port,
			From: testFromAddress,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		err = adapter.Send(context.Background(), validMessage())
		if err == nil || !strings.Contains(err.Error(), "send mail: rcpt_to:") {
			t.Fatalf("Send() err = %v, want wrapped rcpt_to error", err)
		}
	})

	t.Run("DATA command rejected returns wrapped error", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		srv.mu.Lock()
		srv.rejectDataCmdMsg = "5.7.1 message refused"
		srv.mu.Unlock()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host: srv.host,
			Port: srv.port,
			From: testFromAddress,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		err = adapter.Send(context.Background(), validMessage())
		if err == nil || !strings.Contains(err.Error(), "send mail: data:") {
			t.Fatalf("Send() err = %v, want wrapped data error", err)
		}
	})

	t.Run("DATA finalization rejected returns wrapped error", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		srv.mu.Lock()
		srv.rejectDataEndMsg = "5.7.1 rejected after dot"
		srv.mu.Unlock()

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host: srv.host,
			Port: srv.port,
			From: testFromAddress,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		err = adapter.Send(context.Background(), validMessage())
		if err == nil || !strings.Contains(err.Error(), "send mail: data_close:") {
			t.Fatalf("Send() err = %v, want wrapped data_close error", err)
		}
	})
}

func TestSMTPMailer_Auth(t *testing.T) {
	t.Parallel()

	t.Run("no auth attempted when username is empty", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		adapter, err := mailer.NewSMTP(mailer.Config{
			Host: srv.host,
			Port: srv.port,
			From: testFromAddress,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		if err := adapter.Send(context.Background(), validMessage()); err != nil {
			t.Fatalf("Send() err = %v", err)
		}
		_, auths, _ := srv.snapshot()
		if len(auths) != 0 {
			t.Fatalf("auth attempts = %d, want 0", len(auths))
		}
	})

	t.Run("PLAIN auth attempted when username is configured", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:     srv.host,
			Port:     srv.port,
			From:     testFromAddress,
			Username: testUsername,
			Password: testPassword,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		if err := adapter.Send(context.Background(), validMessage()); err != nil {
			t.Fatalf("Send() err = %v", err)
		}
		_, auths, _ := srv.snapshot()
		if len(auths) != 1 {
			t.Fatalf("auth attempts = %d, want 1", len(auths))
		}
		if auths[0].Mechanism != "PLAIN" || auths[0].Username != testUsername || auths[0].Password != testPassword {
			t.Fatalf("auth attempt = %+v, want PLAIN with configured credentials", auths[0])
		}
	})

	t.Run("auth failure wraps ErrAuthFailed and never leaks password even when server echoes it", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		srv.mu.Lock()
		// Simulate a misbehaving server echoing the submitted password in its 535 reply.
		srv.rejectAuthMsg = "5.7.8 invalid password " + testPassword
		srv.mu.Unlock()

		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:     srv.host,
			Port:     srv.port,
			From:     testFromAddress,
			Username: testUsername,
			Password: testPassword,
			Logger:   logger,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		err = adapter.Send(context.Background(), validMessage())
		if !errors.Is(err, mailer.ErrAuthFailed) {
			t.Fatalf("Send() err = %v, want ErrAuthFailed", err)
		}
		if strings.Contains(err.Error(), testPassword) {
			t.Fatalf("Send() error leaked password: %q", err.Error())
		}
		if strings.Contains(logBuf.String(), testPassword) {
			t.Fatalf("log output leaked password: %s", logBuf.String())
		}
	})
}

func TestSMTPMailer_LogsNeverContainRawRecipient(t *testing.T) {
	t.Parallel()

	t.Run("successful send logs masked recipient only", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:   srv.host,
			Port:   srv.port,
			From:   testFromAddress,
			Logger: logger,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		if err := adapter.Send(context.Background(), validMessage()); err != nil {
			t.Fatalf("Send() err = %v", err)
		}

		logged := logBuf.String()
		if logged == "" {
			t.Fatal("expected structured log entry on send")
		}
		if strings.Contains(logged, testRecipientAddress) {
			t.Fatalf("log output leaked raw recipient address: %s", logged)
		}
		if !strings.Contains(logged, testRecipientMasked) {
			t.Fatalf("log output missing masked recipient %q: %s", testRecipientMasked, logged)
		}
	})

	t.Run("failed send scrubs recipient even when SMTP server echoes it", func(t *testing.T) {
		srv := startFakeSMTPServer(t)
		srv.mu.Lock()
		srv.rejectRcptMsg = "5.1.1 <" + testRecipientAddress + ">: Recipient address rejected"
		srv.mu.Unlock()

		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		adapter, err := mailer.NewSMTP(mailer.Config{
			Host:   srv.host,
			Port:   srv.port,
			From:   testFromAddress,
			Logger: logger,
		})
		if err != nil {
			t.Fatalf("NewSMTP: %v", err)
		}

		if err := adapter.Send(context.Background(), validMessage()); err == nil {
			t.Fatal("Send() err = nil, want recipient rejection error")
		}

		logged := logBuf.String()
		if strings.Contains(logged, testRecipientAddress) {
			t.Fatalf("log output leaked raw recipient address from SMTP reply: %s", logged)
		}
		if !strings.Contains(logged, testRecipientMasked) {
			t.Fatalf("log output missing masked recipient %q: %s", testRecipientMasked, logged)
		}
	})
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "standard address", in: testRecipientAddress, want: testRecipientMasked},
		{name: "display name stripped", in: testRecipientDisplay, want: testRecipientMasked},
		{name: "single char local part", in: "a@example.test", want: "***@example.test"},
		{name: "invalid address", in: "not-an-email", want: "<redacted>"},
		{name: "empty string", in: "", want: "<redacted>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mailer.MaskEmail(tt.in); got != tt.want {
				t.Errorf("MaskEmail(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func parseMultipartMessage(t *testing.T, raw []byte) (string, string, mail.Header) {
	t.Helper()

	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("mime.ParseMediaType: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("Content-Type mediaType = %q, want multipart/alternative", mediaType)
	}

	mr := multipart.NewReader(parsed.Body, params["boundary"])
	var textPart, htmlPart string
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("multipart NextPart: %v", err)
		}
		ct := part.Header.Get("Content-Type")
		bodyBytes, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("read part %s: %v", ct, err)
		}
		switch {
		case strings.HasPrefix(ct, "text/plain"):
			textPart = string(bodyBytes)
		case strings.HasPrefix(ct, "text/html"):
			htmlPart = string(bodyBytes)
		}
	}
	if textPart == "" || htmlPart == "" {
		t.Fatalf("missing multipart parts: text=%q html=%q", textPart, htmlPart)
	}
	return textPart, htmlPart, parsed.Header
}
