// Package httpapi builds the HTTP router of apps/api.
//
// Each bounded context registers its own routes through Mount(...); apps/api
// only composes them.
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	paymentshttp "github.com/iris-gate/iris-gate/libs/payments/infra/httpapi"
)

type Mounters struct {
	Identity identityhttp.Deps
	Payments paymentshttp.Deps
}

func NewRouter(m Mounters) http.Handler {
	r := chi.NewRouter()
	// No client-IP middleware: nothing reads the client address, and chi's
	// RealIP trusts spoofable forwarding headers (deprecated since chi v5.5.0).
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz)

	identityhttp.Mount(r, m.Identity)
	paymentshttp.Mount(r, m.Payments)

	return r
}
