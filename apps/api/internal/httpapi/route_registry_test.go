package httpapi_test

import "net/http"

// routeScope classifies whether a route belongs to a merchant tenant or is
// intentionally outside tenant isolation.
//
// phase 08 extends this registry; do not create a second list (for example,
// add a Permission field and additional scopes such as public or platform).
type routeScope int

const (
	// merchantScoped marks routes whose handler scopes every resource access to
	// the authenticated merchant (group: tenantRoutes).
	merchantScoped routeScope = iota
	// notTenantScoped marks routes that operate outside any tenant context;
	// every entry with this scope requires a non-empty Reason.
	notTenantScoped
)

// routeEntry describes one (Method, Pattern) pair mounted on the API router.
//
// Pattern must match the exact spelling reported by chi.Walk: keep chi path
// parameter braces (such as /v1/payment-intents/{id}) and preserve trailing
// slashes if registered with one.
type routeEntry struct {
	Method  string
	Pattern string
	Scope   routeScope
	Reason  string // required when Scope is notTenantScoped; empty for merchantScoped
}

// routeRegistry is the single flat classification table of every HTTP route
// mounted by apps/api.
//
// phase 08 extends this registry; do not create a second list.
//
// Groups:
//   - tenantRoutes (Scope: merchantScoped)
//   - notTenantScoped (Scope: notTenantScoped, Reason required)
var routeRegistry = []routeEntry{
	// tenantRoutes (merchantScoped)
	{Method: http.MethodGet, Pattern: "/v1/me", Scope: merchantScoped},
	{Method: http.MethodGet, Pattern: "/v1/payment-intents", Scope: merchantScoped},
	{Method: http.MethodPost, Pattern: "/v1/payment-intents", Scope: merchantScoped},
	{Method: http.MethodGet, Pattern: "/v1/payment-intents/{id}", Scope: merchantScoped},
	{Method: http.MethodPost, Pattern: "/v1/payment-intents/{id}/authorize", Scope: merchantScoped},
	{Method: http.MethodPost, Pattern: "/v1/payment-intents/{id}/capture", Scope: merchantScoped},

	// notTenantScoped (Reason required)
	{Method: http.MethodGet, Pattern: "/healthz", Scope: notTenantScoped, Reason: "liveness probe, no tenant context"},
	{Method: http.MethodGet, Pattern: "/readyz", Scope: notTenantScoped, Reason: "readiness probe, no tenant context"},
}
