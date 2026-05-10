// Package httpapi monta o router HTTP do apps/api.
//
// Cada bounded context registra suas próprias rotas via Mount(...) — apps/api
// só compõe. Health probes vivem aqui pois são da delivery, não de domínio.
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
)

type Mounters struct {
	Identity identityhttp.Deps
}

func NewRouter(m Mounters) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz)

	identityhttp.Mount(r, m.Identity)

	return r
}
