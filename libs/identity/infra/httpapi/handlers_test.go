package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
	"github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
)

// ---------- in-memory fakes (cópia das fixtures de application) ----------

type fakeMerchantRepo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.Merchant
	byDoc map[string]uuid.UUID
}

func newFakeMerchantRepo() *fakeMerchantRepo {
	return &fakeMerchantRepo{byID: map[uuid.UUID]domain.Merchant{}, byDoc: map[string]uuid.UUID{}}
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
func (r *fakeMerchantRepo) GetByDocument(_ context.Context, d string) (domain.Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byDoc[d]
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
	return &fakeAPIKeyRepo{byID: map[uuid.UUID]domain.APIKey{}, byPrefix: map[string]uuid.UUID{}}
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
func (r *fakeAPIKeyRepo) GetByPrefix(_ context.Context, p string) (domain.APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byPrefix[p]
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

type fakeHasher struct{}

func (fakeHasher) Hash(s string) (string, error) {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:]), nil
}
func (fakeHasher) Verify(s, h string) (bool, error) {
	got, _ := fakeHasher{}.Hash(s)
	return got == h, nil
}

type seqSecrets struct {
	mu sync.Mutex
	n  int
}

func (s *seqSecrets) NewToken() (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return "pfx" + strconv.Itoa(s.n), "sec" + strconv.Itoa(s.n), nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

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

// ---------- Test fixture ----------

type rig struct {
	t      *testing.T
	server *httptest.Server
	keys   *fakeAPIKeyRepo
}

func newRig(t *testing.T) (*rig, application.IssueAPIKeyOutput) {
	t.Helper()

	merchants := newFakeMerchantRepo()
	keys := newFakeAPIKeyRepo()
	hasher := fakeHasher{}
	clock := fixedClock{t: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)}
	ids := &seqIDs{}
	secrets := &seqSecrets{}

	createMerchant := application.NewCreateMerchantUseCase(merchants, clock, ids)
	issueAPIKey := application.NewIssueAPIKeyUseCase(merchants, keys, hasher, secrets, clock, ids)
	revokeAPIKey := application.NewRevokeAPIKeyUseCase(keys, clock)
	auth := application.NewAuthenticateAPIKeyUseCase(keys, merchants, hasher, clock)

	r := chi.NewRouter()
	httpapi.Mount(r, httpapi.Deps{
		CreateMerchant: createMerchant,
		IssueAPIKey:    issueAPIKey,
		RevokeAPIKey:   revokeAPIKey,
		Authenticate:   auth,
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Bootstrap: cria merchant + emite api key via use cases.
	m, err := createMerchant.Execute(context.Background(), application.CreateMerchantInput{
		LegalName:    "ACME",
		Document:     "12345678000199",
		ContactEmail: "ops@acme.test",
	})
	if err != nil {
		t.Fatalf("seed merchant: %v", err)
	}
	out, err := issueAPIKey.Execute(context.Background(), application.IssueAPIKeyInput{MerchantID: m.ID})
	if err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	return &rig{t: t, server: srv, keys: keys}, out
}

func (r *rig) get(t *testing.T, path, token string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, r.server.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := r.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// ---------- Tests do critério ----------

func TestAuth_ValidKey_Returns200(t *testing.T) {
	t.Parallel()
	r, issued := newRig(t)

	code, body := r.get(t, "/v1/me", issued.Token)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
}

func TestAuth_InvalidKey_Returns401(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing":          "",
		"malformed":        "not-a-token",
		"wrong scheme":     "wrong_pfx.sec",
		"wrong secret":     "irk_pfx1.WRONG_SECRET",
		"unknown prefix":   "irk_doesnotexist.whatever",
	}
	for name, token := range cases {
		token := token
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, _ := newRig(t)
			code, _ := r.get(t, "/v1/me", token)
			if code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", code)
			}
		})
	}
}

func TestAuth_RevokedKey_Returns403(t *testing.T) {
	t.Parallel()
	r, issued := newRig(t)

	// Revoga via fake direto (poderia também via POST /v1/api-keys/{id}/revoke).
	if _, err := r.keys.Revoke(context.Background(), issued.APIKey.ID, time.Now()); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	code, body := r.get(t, "/v1/me", issued.Token)
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", code, body)
	}
}

func TestAuth_RevokedKey_Via_HTTP_Endpoint_Returns403(t *testing.T) {
	t.Parallel()
	r, issued := newRig(t)

	// Revoga via endpoint público.
	revokeURL := r.server.URL + "/v1/api-keys/" + issued.APIKey.ID.String() + "/revoke"
	resp, err := http.Post(revokeURL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke endpoint: err=%v status=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	code, _ := r.get(t, "/v1/me", issued.Token)
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestProblemBody_OnInvalidKey(t *testing.T) {
	t.Parallel()
	r, _ := newRig(t)

	code, body := r.get(t, "/v1/me", "irk_x.y")
	if code != http.StatusUnauthorized {
		t.Fatalf("code = %d", code)
	}
	var p map[string]any
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("body não é JSON válido: %s", body)
	}
	if p["status"].(float64) != 401 {
		t.Errorf("problem.status = %v, want 401", p["status"])
	}
	if p["title"] == "" {
		t.Errorf("problem.title vazio")
	}
}
