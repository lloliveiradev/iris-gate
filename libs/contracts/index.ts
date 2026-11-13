// Re-exports of the types generated from the OpenAPI specs.
// Do not edit `generated/`. Run `make gen-contracts` at the root after changing `http/*.openapi.yaml`.

import type * as IdentityRaw from './generated/ts/identity.js';
import type * as PaymentsRaw from './generated/ts/payments.js';
import type * as FraudInternalRaw from './generated/ts/fraud-internal.js';

export namespace Identity {
  type Schemas = IdentityRaw.components['schemas'];

  export type Merchant = Schemas['Merchant'];
  export type MerchantStatus = Schemas['MerchantStatus'];
  export type Problem = Schemas['Problem'];

  export type Paths = IdentityRaw.paths;
}

export namespace Payments {
  type Schemas = PaymentsRaw.components['schemas'];

  export type Money = Schemas['Money'];
  export type CreatePaymentIntentRequest = Schemas['CreatePaymentIntentRequest'];
  export type PaymentIntent = Schemas['PaymentIntent'];
  export type PaymentIntentStatus = Schemas['PaymentIntentStatus'];
  export type Problem = Schemas['Problem'];

  export type Paths = PaymentsRaw.paths;
}

export namespace FraudInternal {
  type Schemas = FraudInternalRaw.components['schemas'];

  export type RiskAssessmentRequest = Schemas['RiskAssessmentRequest'];
  export type RiskAssessmentResponse = Schemas['RiskAssessmentResponse'];
  export type RiskReason = Schemas['RiskReason'];
  export type RiskReasonCode = Schemas['RiskReasonCode'];
  export type RiskDecision = Schemas['RiskDecision'];
  export type Problem = Schemas['Problem'];

  export type Paths = FraudInternalRaw.paths;
}
