// Package acquirer contém adapters para adquirentes. O FakeAcquirer aprova
// tudo e é usado em dev/tests; o adapter real (Stone, Cielo, etc.) entra
// em PR subsequente quando houver credenciais sandbox.
package acquirer

import (
	"context"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
)

type Fake struct{}

func NewFake() Fake { return Fake{} }

func (Fake) Authorize(_ context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	return application.AuthorizeOutput{
		AcquirerRef: "fake_" + uuid.New().String(),
		Approved:    true,
		Reason:      "fake-acquirer-always-approves",
	}, nil
}
