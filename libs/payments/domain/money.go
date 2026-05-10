package domain

import (
	"fmt"
	"regexp"
)

// Money é um value object imutável: valor inteiro em menor unidade da moeda
// (ex.: centavos pra BRL/USD) + currency ISO-4217 alpha-3.
//
// Operações em Money exigem currencies iguais — somar BRL com USD é erro
// explícito, não fallback silencioso.
type Money struct {
	AmountCents int64
	Currency    string
}

var currencyRE = regexp.MustCompile(`^[A-Z]{3}$`)

// NewMoney constrói e valida.
func NewMoney(amount int64, currency string) (Money, error) {
	if amount <= 0 {
		return Money{}, fmt.Errorf("%w: amount must be > 0", ErrValidation)
	}
	if !currencyRE.MatchString(currency) {
		return Money{}, fmt.Errorf("%w: currency must match ISO-4217 alpha-3 (got %q)", ErrValidation, currency)
	}
	return Money{AmountCents: amount, Currency: currency}, nil
}

func (m Money) IsZero() bool { return m.AmountCents == 0 && m.Currency == "" }

// Equals compara amount + currency.
func (m Money) Equals(o Money) bool {
	return m.AmountCents == o.AmountCents && m.Currency == o.Currency
}

func (m Money) String() string {
	return fmt.Sprintf("%d %s", m.AmountCents, m.Currency)
}
