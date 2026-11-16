-- Phase 10 (task 10.3): anti-fraud data model of to-be section 8.6 (closes D6).
--
-- * Amounts become BIGINT minor units.
-- * card_profiles holds the blocked flag that replaces Account.isSuspicious.
-- * payment_outcomes holds the attempts, declines and captures the feature
--   snapshot is read from; scoring records `attempted`.
-- * RiskAssessment stores the feature snapshot the rules saw.
-- * Account and InvoiceAttempt are dropped: nothing reads them since the Kafka
--   request consumer was retired, and their account refs do not map to card
--   fingerprints, so their rows are not carried over.

-- CreateEnum
CREATE TYPE "payment_outcome_kind" AS ENUM ('attempted', 'declined', 'captured');

-- AlterTable
ALTER TABLE "RiskAssessment" ADD COLUMN     "features_snapshot" JSONB,
ALTER COLUMN "amountCents" SET DATA TYPE BIGINT;

-- DropTable
DROP TABLE "Account";

-- DropTable
DROP TABLE "InvoiceAttempt";

-- CreateTable
CREATE TABLE "card_profiles" (
    "card_fingerprint" TEXT NOT NULL,
    "blocked" BOOLEAN NOT NULL DEFAULT false,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(3) NOT NULL,

    CONSTRAINT "card_profiles_pkey" PRIMARY KEY ("card_fingerprint")
);

-- CreateTable
CREATE TABLE "payment_outcomes" (
    "payment_intent_id" TEXT NOT NULL,
    "outcome" "payment_outcome_kind" NOT NULL,
    "merchant_id" TEXT NOT NULL,
    "card_fingerprint" TEXT NOT NULL,
    "card_brand" TEXT,
    "customer_ref" TEXT,
    "amount_cents" BIGINT NOT NULL,
    "currency" CHAR(3) NOT NULL,
    "occurred_at" TIMESTAMPTZ(3) NOT NULL,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "payment_outcomes_pkey" PRIMARY KEY ("payment_intent_id","outcome")
);

-- CreateIndex
CREATE INDEX "payment_outcomes_card_fingerprint_outcome_occurred_at_idx" ON "payment_outcomes"("card_fingerprint", "outcome", "occurred_at");

-- Rule rows whose code the registry does not know make startup throw
-- "unknown rule code", so only the four registered rules are kept. No seed is
-- added; a future seed must keep BLOCKED_CARD enabled with weight >= 0.7.
DELETE FROM "FraudRule"
WHERE "code" NOT IN ('BLOCKED_CARD', 'UNUSUAL_AMOUNT', 'VELOCITY', 'DECLINE_BURST');
