package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// RequireAPIKey reads `Authorization: Bearer <token>`, authenticates it through
// the use case and puts the merchant and API key in the request context.
//
// Error mapping (phase 2 acceptance criterion):
//   - missing or malformed header → 401
//   - invalid token or hash mismatch → 401
//   - revoked API key → 403
func RequireAPIKey(uc *application.AuthenticateAPIKeyUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				problem(w, r, http.StatusUnauthorized, "missing_credentials", "Authorization header missing or malformed")
				return
			}

			out, err := uc.Execute(r.Context(), token)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrAPIKeyInvalid):
					problem(w, r, http.StatusUnauthorized, "invalid_credentials", err.Error())
				case errors.Is(err, domain.ErrAPIKeyRevoked):
					problem(w, r, http.StatusForbidden, "revoked", err.Error())
				case errors.Is(err, domain.ErrMerchantNotFound):
					problem(w, r, http.StatusUnauthorized, "merchant_not_found", err.Error())
				default:
					slog.LogAttrs(r.Context(), slog.LevelError, "auth.unhandled", slog.Any("err", err))
					problem(w, r, http.StatusInternalServerError, "internal", "internal error")
				}
				return
			}

			ctx := contextWithMerchant(r.Context(), out.Merchant, out.APIKey)
			slog.LogAttrs(ctx, slog.LevelDebug, "auth.ok",
				slog.String("merchant_id", out.Merchant.ID.String()),
				slog.String("api_key_id", out.APIKey.ID.String()),
			)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(v[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}
