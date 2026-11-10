// Package hasher implements Hasher (an application-layer port) with Argon2id,
// encoded as a PHC string compatible with `passlib`/`libpwquality`/etc.
package hasher

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params controls the Argon2id cost. Defaults follow OWASP 2024 (m=64MB, t=1, p=4).
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

func DefaultParams() Params {
	return Params{
		Memory:      64 * 1024,
		Iterations:  1,
		Parallelism: 4,
		SaltLen:     16,
		KeyLen:      32,
	}
}

type Argon2 struct {
	p Params
}

func New(p Params) *Argon2 { return &Argon2{p: p} }

func NewDefault() *Argon2 { return New(DefaultParams()) }

func (h *Argon2) Hash(secret string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	digest := argon2.IDKey([]byte(secret), salt, h.p.Iterations, h.p.Memory, h.p.Parallelism, h.p.KeyLen)
	return encodePHC(h.p, salt, digest), nil
}

func (h *Argon2) Verify(secret, encoded string) (bool, error) {
	p, salt, digest, err := decodePHC(encoded)
	if err != nil {
		return false, err
	}
	cmp := argon2.IDKey([]byte(secret), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)
	return subtle.ConstantTimeCompare(cmp, digest) == 1, nil
}

func encodePHC(p Params, salt, digest []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	)
}

func decodePHC(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<digest>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("hasher: bad PHC format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("hasher: unsupported version: %s", parts[2])
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return Params{}, nil, nil, fmt.Errorf("hasher: bad params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("hasher: bad salt: %w", err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("hasher: bad digest: %w", err)
	}
	p.SaltLen = uint32(len(salt))
	p.KeyLen = uint32(len(digest))
	return p, salt, digest, nil
}

// SecretGenerator produces a new (prefix, secret) pair on every call.
//
//	prefix = 16 hex chars (8 random bytes), public and indexed.
//	secret = 32 base64url chars (24 random bytes), hashed with Argon2id.
type SecretGenerator struct{}

func (SecretGenerator) NewToken() (string, string, error) {
	pBytes := make([]byte, 8)
	sBytes := make([]byte, 24)
	if _, err := rand.Read(pBytes); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(sBytes); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(pBytes), base64.RawURLEncoding.EncodeToString(sBytes), nil
}
