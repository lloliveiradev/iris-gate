package actor_test

import (
	"errors"
	"testing"

	"github.com/iris-gate/iris-gate/libs/shared/actor"
)

// Fake ids shared by the tests.
const (
	fakeUserID         = "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b"
	fakeAPIKeyID       = "0b9e8d7c-6a5b-4c3d-9e2f-1a0b9c8d7e6f"
	fakePlatformUserID = "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f"
	fakeSystemID       = "review-timeout"
	fakeRequestID      = "req-0001"
	unknownType        = actor.Type("robot")
)

// U-ACT1: every actor type builds, and the fields are preserved.
func TestNewAcceptsAllActorTypes(t *testing.T) {
	tests := []struct {
		name      string
		actorType actor.Type
		id        string
	}{
		{"user", actor.TypeUser, fakeUserID},
		{"api_key", actor.TypeAPIKey, fakeAPIKeyID},
		{"platform_user", actor.TypePlatformUser, fakePlatformUserID},
		{"system", actor.TypeSystem, fakeSystemID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actor.New(tt.actorType, tt.id, fakeRequestID)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			want := actor.Actor{Type: tt.actorType, ID: tt.id, RequestID: fakeRequestID}
			if got != want {
				t.Errorf("New() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNewKeepsEmptyRequestID(t *testing.T) {
	got, err := actor.New(actor.TypeSystem, fakeSystemID, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got.RequestID != "" {
		t.Errorf("RequestID = %q, want empty", got.RequestID)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		actorType actor.Type
		id        string
		wantErr   error
	}{
		{"unknown type", unknownType, fakeUserID, actor.ErrUnknownType},
		{"empty type", actor.Type(""), fakeUserID, actor.ErrUnknownType},
		{"empty id user", actor.TypeUser, "", actor.ErrEmptyID},
		{"empty id system", actor.TypeSystem, "", actor.ErrEmptyID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actor.New(tt.actorType, tt.id, fakeRequestID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() error = %v, want %v", err, tt.wantErr)
			}
			if got != (actor.Actor{}) {
				t.Errorf("New() = %+v, want zero Actor on error", got)
			}
		})
	}
}
