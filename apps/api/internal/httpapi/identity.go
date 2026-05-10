package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/contracts/generated/go/identity"
)

// CreateMerchant é um stub que valida o contrato gerado de OpenAPI.
// Implementação real entra na Fase 2 (Identity).
func CreateMerchant(w http.ResponseWriter, r *http.Request) {
	var req identity.CreateMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_body", "request body inválido")
		return
	}

	resp := identity.Merchant{
		Id:           openapi_types.UUID(uuid.New()),
		LegalName:    req.LegalName,
		Document:     req.Document,
		ContactEmail: req.ContactEmail,
		Status:       identity.Pending,
		CreatedAt:    time.Now().UTC(),
	}
	writeJSONValue(w, http.StatusCreated, resp)
}

func writeJSONValue(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, code int, title, detail string) {
	p := identity.Problem{
		Type:   "about:blank",
		Title:  title,
		Status: code,
		Detail: &detail,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(p)
}
