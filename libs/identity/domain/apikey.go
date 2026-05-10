package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type APIKeyStatus string

const (
	APIKeyActive  APIKeyStatus = "active"
	APIKeyRevoked APIKeyStatus = "revoked"
)

// APIKey é o registro persistido. O segredo em texto-claro NUNCA é guardado;
// só `Hash` (PHC string Argon2id) é armazenado.
type APIKey struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	Prefix     string
	Hash       string
	Status     APIKeyStatus
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

func (k APIKey) IsRevoked() bool { return k.Status == APIKeyRevoked }

// TokenPrefix é o prefixo público inserido no token entregue ao cliente.
// Convém manter curto pra busca indexada e descartável (sem entropia secreta).
const TokenPrefixLen = 16

// TokenScheme é o prefixo identificador do projeto.
const TokenScheme = "irk"

// FormatToken devolve o token completo no formato `irk_<prefix>.<secret>`.
func FormatToken(prefix, secret string) string {
	return fmt.Sprintf("%s_%s.%s", TokenScheme, prefix, secret)
}

// ParseToken separa um token nos seus componentes.
// Devolve ErrAPIKeyInvalid se o formato não bater.
func ParseToken(token string) (prefix, secret string, err error) {
	const sep = "."

	tail, ok := strings.CutPrefix(token, TokenScheme+"_")
	if !ok {
		return "", "", fmt.Errorf("%w: bad scheme", ErrAPIKeyInvalid)
	}
	prefix, secret, ok = strings.Cut(tail, sep)
	if !ok || prefix == "" || secret == "" {
		return "", "", fmt.Errorf("%w: missing prefix or secret", ErrAPIKeyInvalid)
	}
	return prefix, secret, nil
}
