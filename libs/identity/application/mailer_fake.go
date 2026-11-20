package application

import (
	"context"
	"fmt"
	"sync"
)

// FakeMailer is an in-memory Mailer test double that records sent messages and
// can be configured to fail. Intended for unit tests of use cases and shared
// Mailer contract tests.
type FakeMailer struct {
	mu       sync.Mutex
	messages []Message
	err      error
}

var _ Mailer = (*FakeMailer)(nil)

// NewFakeMailer returns an empty FakeMailer ready for use in tests.
func NewFakeMailer() *FakeMailer {
	return &FakeMailer{}
}

// Send validates msg, honors ctx cancellation, optionally fails with the
// configured error, and otherwise records msg in memory.
func (f *FakeMailer) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := ValidateMessage(msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return fmt.Errorf("send mail: %w", f.err)
	}
	f.messages = append(f.messages, msg)
	return nil
}

// FailWith configures subsequent Send calls to fail with err. Pass nil to clear.
func (f *FakeMailer) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Messages returns a copy of all successfully recorded messages in send order.
func (f *FakeMailer) Messages() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.messages))
	copy(out, f.messages)
	return out
}

// Reset clears all recorded messages and any configured error.
func (f *FakeMailer) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = nil
	f.err = nil
}
