package mailer_test

import (
	"github.com/iris-gate/iris-gate/libs/identity/application"
)

// Shared fake, non-secret test fixtures. Values are obviously synthetic so log
// and error redaction assertions can check for their absence reliably.
const (
	testLocalHost        = "127.0.0.1"
	testFromAddress      = "no-reply@iris-gate.local"
	testFromDisplay      = "Iris Gate <no-reply@iris-gate.local>"
	testRecipientAddress = "jane.doe@example.test"
	testRecipientDisplay = "Jane Doe <jane.doe@example.test>"
	testRecipientMasked  = "j***@example.test"
	testSubject          = "Verify your Iris Gate account"
	testUTF8Subject      = "Iris Gate — account notice"
	testTextBody         = "Hello,\n\nPlease confirm your account to continue."
	testHTMLFragment     = "<p>Hello,</p><p>Please confirm your account to continue.</p>"
	testFullHTMLDoc      = "<!DOCTYPE html><html lang=\"en\"><body><p>Full custom HTML</p></body></html>"
	testUsername         = "smtp-test-user"
	testPassword         = "smtp-test-secret-credential"
	testInjectedAddress  = "evil@example.test"
)

func validMessage() application.Message {
	return application.Message{
		To:      testRecipientAddress,
		Subject: testSubject,
		Text:    testTextBody,
		HTML:    testHTMLFragment,
	}
}
