// Package idempotency is an HTTP middleware backed by a Store (Postgres in
// production).
//
// Behavior (Stripe-style idempotency), per (merchant, scope, key):
//
//  1. No authenticated merchant → 401.
//  2. No header → 400. A key shorter than 8 or longer than 128 bytes (the
//     payments contract bounds) → 400 invalid_idempotency_key.
//  3. Body over 64 KiB → 413 request_too_large; any other read failure → 400.
//     Both answer before the key is reserved.
//  4. New key → the key is reserved in the store before the handler runs, so
//     concurrent first requests cannot both run it. A 2xx response is stored
//     on the reservation; any other status, or a panic, releases the key so
//     the client can retry. Either happens before the client sees the
//     response. A handler response over 1 MiB is not stored: the key stays
//     reserved, as when storing a 2xx response fails (the handler already ran,
//     so a retry must not run it again), and the client gets a 500.
//  5. Same key and body, response stored → the stored response is replayed
//     without running the handler.
//  6. Same key, different body → 409 idempotency_conflict.
//  7. Same key and body while the first request still runs → 409
//     idempotency_in_progress. The client retries later and gets the replay
//     (or runs the request, if the first one failed and released the key).
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
)

// Store is the port the middleware consumes (Postgres in production). Every
// call is scoped to one (merchantID, scope, key).
type Store interface {
	// Reserve claims the key for a request whose body hashes to requestHash.
	// It is atomic: of several concurrent calls for a free key, exactly one
	// gets (nil, nil) and holds the reservation. Every other call gets the
	// record stored for the key, which is InProgress until the holder
	// completes it.
	Reserve(ctx context.Context, merchantID uuid.UUID, scope, key string, requestHash []byte) (*Record, error)
	// Complete stores the response on the reservation held for the key.
	Complete(ctx context.Context, merchantID uuid.UUID, scope, key string, status int, body []byte) error
	// Release removes the reservation held for the key, so the key can be
	// used again. It never removes a completed record.
	Release(ctx context.Context, merchantID uuid.UUID, scope, key string) error
}

// Record is what the store holds for a key. InProgress is true while the
// request that reserved the key runs; the response fields are empty until it
// completes.
type Record struct {
	RequestHash    []byte
	InProgress     bool
	ResponseStatus int
	ResponseBody   []byte
}

const (
	keyHeader    = "Idempotency-Key"
	replayHeader = "Idempotent-Replay"

	noPrincipalTitle     = "no_principal"
	missingKeyTitle      = "missing_idempotency_key"
	invalidKeyTitle      = "invalid_idempotency_key"
	invalidBodyTitle     = "invalid_body"
	requestTooLargeTitle = "request_too_large"
	conflictTitle        = "idempotency_conflict"
	inProgressTitle      = "idempotency_in_progress"
	internalTitle        = "internal"

	// minKeyLength and maxKeyLength bound the Idempotency-Key header, as the
	// payments OpenAPI contract does.
	minKeyLength = 8
	maxKeyLength = 128

	// maxRequestBodyBytes caps the request body the middleware buffers to hash
	// it, so a client cannot make it hold an arbitrary amount of memory. A
	// payment intent body is a few hundred bytes.
	maxRequestBodyBytes = 64 << 10

	// maxResponseBodyBytes caps the handler response the middleware buffers to
	// store it, which bounds the size of a stored (and replayed) response.
	maxResponseBodyBytes = 1 << 20

	// storeTimeout bounds each store call. The calls run detached from the
	// client's cancellation (see storeContext), so this is what stops one
	// that hangs.
	storeTimeout = 5 * time.Second
)

// errResponseTooLarge is what the recorder answers a handler that writes past
// maxResponseBodyBytes.
var errResponseTooLarge = errors.New("idempotency: handler response exceeds the buffer limit")

// Middleware returns an http.Handler middleware for the given route `scope`
// (e.g. "create_payment_intent"). Records are keyed by (merchant, scope, key):
// the merchant comes from the authenticated principal, so it must be mounted
// after the authentication middleware. Without a principal it answers 401.
func Middleware(store Store, scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			merchant, ok := identityhttp.MerchantFromContext(r.Context())
			if !ok {
				writeProblem(w, http.StatusUnauthorized, noPrincipalTitle, "authenticated merchant required")
				return
			}

			key := r.Header.Get(keyHeader)
			if key == "" {
				writeProblem(w, http.StatusBadRequest, missingKeyTitle, "Idempotency-Key header required")
				return
			}
			if n := len(key); n < minKeyLength || n > maxKeyLength {
				detail := fmt.Sprintf("Idempotency-Key must be %d to %d characters", minKeyLength, maxKeyLength)
				writeProblem(w, http.StatusBadRequest, invalidKeyTitle, detail)
				return
			}

			body, ok := readBody(w, r)
			if !ok {
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := sha256.Sum256(body)
			e := entry{store: store, merchantID: merchant.ID, scope: scope, key: key}

			existing, err := e.reserve(r.Context(), hash[:])
			if err != nil {
				slog.LogAttrs(r.Context(), slog.LevelError, "idempotency.reserve", slog.Any("err", err))
				writeProblem(w, http.StatusInternalServerError, internalTitle, "idempotency reservation failed")
				return
			}
			if existing != nil {
				answerExisting(w, existing, hash[:])
				return
			}
			e.serve(w, r, next)
		})
	}
}

// readBody reads the request body, up to maxRequestBodyBytes. When it cannot,
// it has already answered (413 over the cap, 400 for any other failure) and ok
// is false. The answers are fixed text: the read error is not echoed.
func readBody(w http.ResponseWriter, r *http.Request) (body []byte, ok bool) {
	capped := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer func() { _ = capped.Close() }()

	body, err := io.ReadAll(capped)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeProblem(w, http.StatusRequestEntityTooLarge, requestTooLargeTitle, "request body exceeds the size limit")
		return nil, false
	case err != nil:
		slog.LogAttrs(r.Context(), slog.LevelWarn, "idempotency.read_body", slog.Any("err", err))
		writeProblem(w, http.StatusBadRequest, invalidBodyTitle, "request body could not be read")
		return nil, false
	}
	return body, true
}

// answerExisting answers a request whose key the store already holds.
func answerExisting(w http.ResponseWriter, existing *Record, requestHash []byte) {
	switch {
	case !bytes.Equal(existing.RequestHash, requestHash):
		writeProblem(w, http.StatusConflict, conflictTitle, "key reused with a different body")
	case existing.InProgress:
		writeProblem(w, http.StatusConflict, inProgressTitle, "a request with this key is still in progress; retry later")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(replayHeader, "true")
		w.WriteHeader(existing.ResponseStatus)
		_, _ = w.Write(existing.ResponseBody)
	}
}

// entry is the store record of one (merchant, scope, key).
type entry struct {
	store      Store
	merchantID uuid.UUID
	scope      string
	key        string
}

// serve runs next while this request holds the reservation. The response is
// buffered: the key is completed (2xx) or released (any other status) before
// the client sees it, so a client that retries right after the answer finds
// the stored response or a free key, never its own reservation. A response
// over maxResponseBodyBytes is not stored: the key stays reserved and the
// client gets a 500.
func (e entry) serve(w http.ResponseWriter, r *http.Request, next http.Handler) {
	settled := false
	defer func() {
		// next panicked: release the key and let the panic go on.
		if !settled {
			e.release(r.Context())
		}
	}()

	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	next.ServeHTTP(rec, r)
	if rec.overflow {
		// The handler already ran and its side effect happened, so the key
		// stays reserved, as when Complete fails: releasing it would let a
		// retry run the handler again.
		settled = true
		slog.LogAttrs(r.Context(), slog.LevelError, "idempotency.response_too_large", slog.Any("err", errResponseTooLarge))
		writeProblem(w, http.StatusInternalServerError, internalTitle, "response too large to store")
		return
	}
	if isSuccess(rec.status) {
		e.complete(r.Context(), rec.status, rec.body.Bytes())
	} else {
		e.release(r.Context())
	}
	settled = true

	w.WriteHeader(rec.status)
	_, _ = w.Write(rec.body.Bytes())
}

func (e entry) reserve(ctx context.Context, requestHash []byte) (*Record, error) {
	ctx, cancel := storeContext(ctx)
	defer cancel()
	return e.store.Reserve(ctx, e.merchantID, e.scope, e.key, requestHash)
}

func (e entry) complete(ctx context.Context, status int, body []byte) {
	ctx, cancel := storeContext(ctx)
	defer cancel()
	if err := e.store.Complete(ctx, e.merchantID, e.scope, e.key, status, body); err != nil {
		// The handler's side effect already happened, so the key stays
		// reserved: releasing it would let a retry run the handler again.
		slog.LogAttrs(ctx, slog.LevelError, "idempotency.complete", slog.Any("err", err))
	}
}

func (e entry) release(ctx context.Context) {
	ctx, cancel := storeContext(ctx)
	defer cancel()
	if err := e.store.Release(ctx, e.merchantID, e.scope, e.key); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "idempotency.release", slog.Any("err", err))
	}
}

// storeContext detaches ctx from the client's cancellation and bounds it by
// storeTimeout. A client that disconnects must not cut a store call halfway:
// a reservation whose commit is interrupted, or a release that never runs,
// would leave the key reserved with no request holding it.
func storeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
}

func isSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

// recorder buffers the handler's status and body, up to maxResponseBodyBytes.
// A write that would pass the cap is refused with errResponseTooLarge and sets
// overflow; the buffer then holds only what fit. Headers go straight to the
// underlying ResponseWriter, which serve writes once the key is settled.
type recorder struct {
	http.ResponseWriter
	status   int
	body     bytes.Buffer
	overflow bool
}

func (r *recorder) WriteHeader(status int) { r.status = status }

func (r *recorder) Write(p []byte) (int, error) {
	if r.overflow || r.body.Len()+len(p) > maxResponseBodyBytes {
		r.overflow = true
		return 0, errResponseTooLarge
	}
	return r.body.Write(p)
}

func writeProblem(w http.ResponseWriter, code int, title, detail string) {
	body := map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": code,
		"detail": detail,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
