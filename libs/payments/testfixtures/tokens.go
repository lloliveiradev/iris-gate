// Package testfixtures holds fictitious values shared by the payments tests.
//
// Everything here is a fake, non-secret fixture. Keep values obviously fake
// and low-entropy (`tok_test_...`) so secret scanners do not mistake them for
// real credentials, and declare each one once instead of repeating literals.
package testfixtures

// Card tokens, grouped by the behavior a test expects from them.
const (
	// TokenApproved is an opaque card token that tests treat as a valid card.
	TokenApproved = "tok_test_visa"
	// TokenDeclined is an opaque card token that tests configure the fake
	// acquirer to decline.
	TokenDeclined = "tok_test_declined"
	// TokenTooShort is below the minimum card token length.
	TokenTooShort = "tok_a"
)

// AcquirerBearer is the fake bearer token the HTTP acquirer tests expect.
const AcquirerBearer = "acquirer_test_bearer"
