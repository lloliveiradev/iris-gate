// Package actor models who performed an action, for audit events and
// attribution (to-be section 4.5). It is part of the shared kernel and
// imports no bounded context.
package actor

import (
	"errors"
	"fmt"
)

// Type is the closed set of actor origins.
type Type string

// The four actor origins. Only these values are valid.
const (
	// TypeUser is a merchant user acting from the dashboard; the ID is the user id.
	TypeUser Type = "user"
	// TypeAPIKey is a merchant integration; the ID is the API key id.
	TypeAPIKey Type = "api_key"
	// TypePlatformUser is a platform staff user acting from the console; the ID is the user id.
	TypePlatformUser Type = "platform_user"
	// TypeSystem is an automatic action; the ID is the job or component name.
	TypeSystem Type = "system"
)

// Domain errors returned by New.
var (
	// ErrUnknownType is returned for a Type outside the closed set.
	ErrUnknownType = errors.New("actor: unknown type")
	// ErrEmptyID is returned when the actor id is empty.
	ErrEmptyID = errors.New("actor: empty id")
)

// Valid reports whether t is one of the four known actor types.
func (t Type) Valid() bool {
	switch t {
	case TypeUser, TypeAPIKey, TypePlatformUser, TypeSystem:
		return true
	default:
		return false
	}
}

// Actor identifies who did something. The ID has no foreign key to any table,
// so history outlives the actor. A system actor carries its component name as
// ID; there is no nil or placeholder actor.
type Actor struct {
	Type      Type
	ID        string
	RequestID string
}

// New builds an Actor. It rejects an unknown type or an empty id and keeps
// requestID as given (it may be empty when there is no request).
func New(actorType Type, id, requestID string) (Actor, error) {
	if !actorType.Valid() {
		return Actor{}, fmt.Errorf("%w: %q", ErrUnknownType, actorType)
	}
	if id == "" {
		return Actor{}, ErrEmptyID
	}
	return Actor{Type: actorType, ID: id, RequestID: requestID}, nil
}
