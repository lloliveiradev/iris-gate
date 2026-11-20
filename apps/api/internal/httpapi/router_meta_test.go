package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/iris-gate/iris-gate/apps/api/internal/httpapi"
	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	paymentsapp "github.com/iris-gate/iris-gate/libs/payments/application"
	paymentshttp "github.com/iris-gate/iris-gate/libs/payments/infra/httpapi"
)

type nopPaymentIntentRepo struct {
	paymentsapp.PaymentIntentRepo
}

func nopMiddleware(next http.Handler) http.Handler {
	return next
}

func nopHandler(http.ResponseWriter, *http.Request) {}

func newFullTestRouter(t *testing.T) chi.Routes {
	t.Helper()

	h := httpapi.NewRouter(httpapi.Mounters{
		Identity: identityhttp.Deps{
			Authenticate: &identityapp.AuthenticateAPIKeyUseCase{},
		},
		Payments: paymentshttp.Deps{
			CreatePaymentIntent: &paymentsapp.CreatePaymentIntentUseCase{},
			AuthorizePayment:    &paymentsapp.AuthorizePaymentUseCase{},
			CapturePayment:      &paymentsapp.CapturePaymentUseCase{},
			Repo:                nopPaymentIntentRepo{},
			AuthRequired:        nopMiddleware,
			IdempotencyMW:       nopMiddleware,
		},
	})

	routes, ok := h.(chi.Routes)
	if !ok {
		t.Fatalf("NewRouter returned %T, want chi.Routes", h)
	}
	return routes
}

type routeKey struct {
	method  string
	pattern string
}

func (k routeKey) String() string {
	return k.method + " " + k.pattern
}

func normalizeRouteKey(method, pattern string) routeKey {
	// chi.Walk preserves the exact registered pattern (including trailing slashes
	// and {param} placeholders), so only surrounding whitespace and method case
	// are normalized here.
	return routeKey{
		method:  strings.ToUpper(strings.TrimSpace(method)),
		pattern: strings.TrimSpace(pattern),
	}
}

func walkRoutes(r chi.Routes) ([]routeKey, error) {
	seen := make(map[routeKey]struct{})
	var out []routeKey

	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		k := normalizeRouteKey(method, route)
		if _, ok := seen[k]; ok {
			return nil
		}
		seen[k] = struct{}{}
		out = append(out, k)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk chi routes: %w", err)
	}

	slices.SortFunc(out, compareRouteKeys)
	return out, nil
}

func compareRouteKeys(a, b routeKey) int {
	if c := strings.Compare(a.pattern, b.pattern); c != 0 {
		return c
	}
	return strings.Compare(a.method, b.method)
}

func checkRouteRegistry(r chi.Routes, entries []routeEntry) ([]string, error) {
	mounted, err := walkRoutes(r)
	if err != nil {
		return nil, err
	}

	mountedSet := make(map[routeKey]struct{}, len(mounted))
	for _, k := range mounted {
		mountedSet[k] = struct{}{}
	}

	registrySet := make(map[routeKey]struct{}, len(entries))
	var problems []string

	for _, e := range entries {
		k := normalizeRouteKey(e.Method, e.Pattern)
		if k.method == "" || k.pattern == "" {
			problems = append(problems, fmt.Sprintf("route registry entry %q %q must have non-empty method and pattern in route_registry_test.go", e.Method, e.Pattern))
			continue
		}

		if _, dup := registrySet[k]; dup {
			problems = append(problems, fmt.Sprintf("duplicate route registry entry %s in route_registry_test.go", k))
			continue
		}
		registrySet[k] = struct{}{}

		switch e.Scope {
		case merchantScoped:
			if strings.TrimSpace(e.Reason) != "" {
				problems = append(problems, fmt.Sprintf("route %s is merchantScoped and must have an empty reason in route_registry_test.go", k))
			}
		case notTenantScoped:
			if strings.TrimSpace(e.Reason) == "" {
				problems = append(problems, fmt.Sprintf("route %s is marked notTenantScoped without a reason in route_registry_test.go", k))
			}
		default:
			problems = append(problems, fmt.Sprintf("route %s has unknown scope %d in route_registry_test.go", k, e.Scope))
		}

		if _, ok := mountedSet[k]; !ok {
			problems = append(problems, fmt.Sprintf("stale route registry entry %s is not mounted on the router; remove it from route_registry_test.go", k))
		}
	}

	for _, k := range mounted {
		if _, ok := registrySet[k]; !ok {
			problems = append(problems, fmt.Sprintf("route %s is not classified. Add it to tenantRoutes (merchant-scoped) or notTenantScoped (with a reason) in route_registry_test.go", k))
		}
	}

	return problems, nil
}

func TestRouterRoutesAreClassified(t *testing.T) {
	t.Parallel()

	problems, err := checkRouteRegistry(newFullTestRouter(t), routeRegistry)
	if err != nil {
		t.Fatalf("checkRouteRegistry error: %v", err)
	}
	if len(problems) > 0 {
		t.Fatalf("route registry mismatches:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestCheckRouteRegistry_UnclassifiedRoute(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Get("/healthz", nopHandler)
	r.Post("/v1/x", nopHandler)

	reg := []routeEntry{
		{Method: http.MethodGet, Pattern: "/healthz", Scope: notTenantScoped, Reason: "liveness"},
	}

	problems, err := checkRouteRegistry(r, reg)
	if err != nil {
		t.Fatalf("checkRouteRegistry error: %v", err)
	}

	want := "route POST /v1/x is not classified. Add it to tenantRoutes (merchant-scoped) or notTenantScoped (with a reason) in route_registry_test.go"
	if !slices.Equal(problems, []string{want}) {
		t.Fatalf("problems = %#v, want [%q]", problems, want)
	}
}

func TestCheckRouteRegistry_NestedRouteAndGroup(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Use(nopMiddleware)
	r.Route("/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(nopMiddleware)
			r.Get("/items/{id}", nopHandler)
		})
	})

	problems, err := checkRouteRegistry(r, nil)
	if err != nil {
		t.Fatalf("checkRouteRegistry error: %v", err)
	}
	want := "route GET /v1/items/{id} is not classified. Add it to tenantRoutes (merchant-scoped) or notTenantScoped (with a reason) in route_registry_test.go"
	if !slices.Equal(problems, []string{want}) {
		t.Fatalf("unclassified nested problems = %#v, want [%q]", problems, want)
	}

	reg := []routeEntry{
		{Method: http.MethodGet, Pattern: "/v1/items/{id}", Scope: merchantScoped},
	}
	problems, err = checkRouteRegistry(r, reg)
	if err != nil {
		t.Fatalf("checkRouteRegistry error: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("classified nested problems = %#v, want none", problems)
	}
}

func TestCheckRouteRegistry_MethodsAndDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("same pattern with two methods requires both entries", func(t *testing.T) {
		t.Parallel()

		r := chi.NewRouter()
		r.Get("/v1/items", nopHandler)
		r.Post("/v1/items", nopHandler)

		reg := []routeEntry{
			{Method: http.MethodGet, Pattern: "/v1/items", Scope: merchantScoped},
		}
		problems, err := checkRouteRegistry(r, reg)
		if err != nil {
			t.Fatalf("checkRouteRegistry error: %v", err)
		}
		want := "route POST /v1/items is not classified. Add it to tenantRoutes (merchant-scoped) or notTenantScoped (with a reason) in route_registry_test.go"
		if !slices.Equal(problems, []string{want}) {
			t.Fatalf("problems = %#v, want [%q]", problems, want)
		}

		reg = append(reg, routeEntry{Method: http.MethodPost, Pattern: "/v1/items", Scope: merchantScoped})
		problems, err = checkRouteRegistry(r, reg)
		if err != nil {
			t.Fatalf("checkRouteRegistry error: %v", err)
		}
		if len(problems) != 0 {
			t.Fatalf("problems = %#v, want none", problems)
		}
	})

	t.Run("same method registered twice on chi collapses to one route", func(t *testing.T) {
		t.Parallel()

		r := chi.NewRouter()
		r.Get("/v1/dup", nopHandler)
		r.Get("/v1/dup", nopHandler)

		reg := []routeEntry{
			{Method: http.MethodGet, Pattern: "/v1/dup", Scope: merchantScoped},
		}
		problems, err := checkRouteRegistry(r, reg)
		if err != nil {
			t.Fatalf("checkRouteRegistry error: %v", err)
		}
		if len(problems) != 0 {
			t.Fatalf("problems = %#v, want none", problems)
		}
	})

	t.Run("duplicate registry entry fails", func(t *testing.T) {
		t.Parallel()

		r := chi.NewRouter()
		r.Get("/v1/dup", nopHandler)

		reg := []routeEntry{
			{Method: http.MethodGet, Pattern: "/v1/dup", Scope: merchantScoped},
			{Method: http.MethodGet, Pattern: "/v1/dup", Scope: merchantScoped},
		}
		problems, err := checkRouteRegistry(r, reg)
		if err != nil {
			t.Fatalf("checkRouteRegistry error: %v", err)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "duplicate") || !strings.Contains(problems[0], "GET /v1/dup") {
			t.Fatalf("problems = %#v, want duplicate error for GET /v1/dup", problems)
		}
	})
}

func TestCheckRouteRegistry_ValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		setup    func(r chi.Router)
		reg      []routeEntry
		wantSubs []string
	}{
		{
			name: "stale entry",
			setup: func(r chi.Router) {
				r.Get("/healthz", nopHandler)
			},
			reg: []routeEntry{
				{Method: http.MethodGet, Pattern: "/healthz", Scope: notTenantScoped, Reason: "liveness"},
				{Method: http.MethodGet, Pattern: "/v1/removed", Scope: merchantScoped},
			},
			wantSubs: []string{"stale", "GET /v1/removed"},
		},
		{
			name: "notTenantScoped without reason",
			setup: func(r chi.Router) {
				r.Get("/healthz", nopHandler)
			},
			reg: []routeEntry{
				{Method: http.MethodGet, Pattern: "/healthz", Scope: notTenantScoped, Reason: "   "},
			},
			wantSubs: []string{"notTenantScoped", "without a reason", "GET /healthz"},
		},
		{
			name: "trailing slash mismatch fails against chi spelling",
			setup: func(r chi.Router) {
				r.Get("/v1/trailing/", nopHandler)
			},
			reg: []routeEntry{
				{Method: http.MethodGet, Pattern: "/v1/trailing", Scope: merchantScoped},
			},
			wantSubs: []string{"stale route registry entry GET /v1/trailing", "route GET /v1/trailing/ is not classified"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := chi.NewRouter()
			tc.setup(r)

			problems, err := checkRouteRegistry(r, tc.reg)
			if err != nil {
				t.Fatalf("checkRouteRegistry error: %v", err)
			}
			joined := strings.Join(problems, "\n")
			for _, sub := range tc.wantSubs {
				if !strings.Contains(joined, sub) {
					t.Fatalf("problems = %#v, want substring %q", problems, sub)
				}
			}
		})
	}
}

func TestCheckRouteRegistry_TrailingSlashAndMiddlewareMatch(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Use(nopMiddleware)
	r.With(nopMiddleware).Get("/v1/trailing/", nopHandler)
	r.With(nopMiddleware).Get("/v1/payment-intents/{id}", nopHandler)

	reg := []routeEntry{
		{Method: http.MethodGet, Pattern: "/v1/trailing/", Scope: merchantScoped},
		{Method: http.MethodGet, Pattern: "/v1/payment-intents/{id}", Scope: merchantScoped},
	}

	problems, err := checkRouteRegistry(r, reg)
	if err != nil {
		t.Fatalf("checkRouteRegistry error: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %#v, want none", problems)
	}
}
