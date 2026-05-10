// Re-exports dos tipos gerados a partir das specs OpenAPI.
// NÃO editar `generated/`. Rodar `make gen-contracts` na raiz após alterar `http/*.openapi.yaml`.

import type * as IdentityRaw from './generated/ts/identity.js';

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
