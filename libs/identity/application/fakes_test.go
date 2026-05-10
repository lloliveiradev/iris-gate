package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// fakeMerchantRepo é um repo in-memory para testes.
type fakeMerchantRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]domain.Merchant
	byDoc   map[string]uuid.UUID
}

func newFakeMerchantRepo() *fakeMerchantRepo {
	return &fakeMerchantRepo{
		byID:  make(map[uuid.UUID]domain.Merchant),
		byDoc: make(map[string]uuid.UUID),
	}
}

func (r *fakeMerchantRepo) Create(_ context.Context, m domain.Merchant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byDoc[m.Document]; ok {
		return domain.ErrDocumentExists
	}
	r.byID[m.ID] = m
	r.byDoc[m.Document] = m.ID
	return nil
}

func (r *fakeMerchantRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byID[id]
	if !ok {
		return domain.Merchant{}, domain.ErrMerchantNotFound
	}
	return m, nil
}

func (r *fakeMerchantRepo) GetByDocument(_ context.Context, doc string) (domain.Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byDoc[doc]
	if !ok {
		return domain.Merchant{}, domain.ErrMerchantNotFound
	}
	return r.byID[id], nil
}

type fakeAPIKeyRepo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]domain.APIKey
	byPrefix map[string]uuid.UUID
}

func newFakeAPIKeyRepo() *fakeAPIKeyRepo {
	return &fakeAPIKeyRepo{
		byID:     make(map[uuid.UUID]domain.APIKey),
		byPrefix: make(map[string]uuid.UUID),
	}
}

func (r *fakeAPIKeyRepo) Create(_ context.Context, k domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byPrefix[k.Prefix]; ok {
		return errors.New("prefix collision")
	}
	r.byID[k.ID] = k
	r.byPrefix[k.Prefix] = k.ID
	return nil
}

func (r *fakeAPIKeyRepo) GetByID(_ context.Context, id uuid.UUID) (domain.APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.byID[id]
	if !ok {
		return domain.APIKey{}, domain.ErrAPIKeyNotFound
	}
	return k, nil
}

func (r *fakeAPIKeyRepo) GetByPrefix(_ context.Context, prefix string) (domain.APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byPrefix[prefix]
	if !ok {
		return domain.APIKey{}, domain.ErrAPIKeyNotFound
	}
	return r.byID[id], nil
}

func (r *fakeAPIKeyRepo) Revoke(_ context.Context, id uuid.UUID, at time.Time) (domain.APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.byID[id]
	if !ok {
		return domain.APIKey{}, domain.ErrAPIKeyNotFound
	}
	k.Status = domain.APIKeyRevoked
	k.RevokedAt = &at
	r.byID[id] = k
	return k, nil
}

func (r *fakeAPIKeyRepo) TouchLastUsed(_ context.Context, id uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.byID[id]
	if !ok {
		return domain.ErrAPIKeyNotFound
	}
	k.LastUsedAt = &at
	r.byID[id] = k
	return nil
}

// Hasher de teste — SHA-256 hex (NÃO usar em prod).
type fakeHasher struct{}

func (fakeHasher) Hash(secret string) (string, error) {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:]), nil
}

func (fakeHasher) Verify(secret, hash string) (bool, error) {
	got, _ := fakeHasher{}.Hash(secret)
	return got == hash, nil
}

// SecretGen determinístico (índices crescentes) — fácil de afirmar em testes.
type seqSecrets struct {
	mu sync.Mutex
	n  int
}

func (s *seqSecrets) NewToken() (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return "prefix" + strconv.Itoa(s.n), "secret" + strconv.Itoa(s.n), nil
}

// Clock fixo.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// IDGen determinístico baseado em uuid.NewMD5 (estável por nome).
type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (g *seqIDs) NewUUID() uuid.UUID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return uuid.NewMD5(uuid.NameSpaceOID, []byte(strconv.Itoa(g.n)))
}

// Sentinel for compiler check.
var (
	_ application.MerchantRepo = (*fakeMerchantRepo)(nil)
	_ application.APIKeyRepo   = (*fakeAPIKeyRepo)(nil)
	_ application.Hasher       = fakeHasher{}
	_ application.SecretGen    = (*seqSecrets)(nil)
	_ application.Clock        = fixedClock{}
	_ application.IDGen        = (*seqIDs)(nil)
)
