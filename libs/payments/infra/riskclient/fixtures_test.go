package riskclient

import (
	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// Fake, non-secret values shared by the riskclient tests. Card-like values are
// obviously fake so the log assertions can search for them.
const (
	testServiceToken    = "svc_test_token"
	testHostPort        = "anti-fraud.test:3001"
	testBaseURL         = "http://" + testHostPort
	testCardFingerprint = "fp_test_card_fingerprint"
	testCardBrand       = "test_brand"
	testCustomerRef     = "cus_test_ref"
	testCurrency        = "BRL"
	// testAmountCents is above 2^31 to prove the amount travels as int64.
	testAmountCents int64 = 3_000_000_000

	testPaymentIntentID = "11111111-1111-4111-8111-111111111111"
	testMerchantID      = "22222222-2222-4222-8222-222222222222"
	testAssessmentID    = "33333333-3333-4333-8333-333333333333"

	// testApproveScore and testRejectScore are exact in float32, so the
	// float32 → float64 conversion does not change them.
	testApproveScore = 12.5
	testRejectScore  = 100
)

func testRequest() application.RiskAssessmentRequest {
	return application.RiskAssessmentRequest{
		PaymentIntentID: uuid.MustParse(testPaymentIntentID),
		MerchantID:      uuid.MustParse(testMerchantID),
		Amount:          domain.Money{AmountCents: testAmountCents, Currency: testCurrency},
		CardFingerprint: testCardFingerprint,
		CardBrand:       testCardBrand,
		CustomerRef:     testCustomerRef,
	}
}

// secretValues must never appear in a log line.
func secretValues() []string {
	return []string{testServiceToken, testCardFingerprint, testCardBrand, testCustomerRef}
}
