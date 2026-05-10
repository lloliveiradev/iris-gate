package domain

import "errors"

var (
	ErrValidation       = errors.New("validation")
	ErrMerchantNotFound = errors.New("merchant not found")
	ErrAPIKeyNotFound   = errors.New("api key not found")
	ErrAPIKeyInvalid    = errors.New("api key invalid")
	ErrAPIKeyRevoked    = errors.New("api key revoked")
	ErrDocumentExists   = errors.New("document already exists")
)
