// Re-exports dos tipos gerados a partir das specs OpenAPI.
// NÃO editar `generated/`. Rodar `make gen-contracts` na raiz após alterar `http/*.openapi.yaml`.

import type * as IdentityRaw from './generated/ts/identity.js';
import type * as PaymentsRaw from './generated/ts/payments.js';

export namespace Identity {
  type Schemas = IdentityRaw.components['schemas'];

  export type CreateMerchantRequest = Schemas['CreateMerchantRequest'];
  export type Merchant = Schemas['Merchant'];
  export type MerchantStatus = Schemas['MerchantStatus'];
  export type IssueApiKeyRequest = Schemas['IssueApiKeyRequest'];
  export type IssueApiKeyResponse = Schemas['IssueApiKeyResponse'];
  export type ApiKey = Schemas['ApiKey'];
  export type ApiKeyStatus = Schemas['ApiKeyStatus'];
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
