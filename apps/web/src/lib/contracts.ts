// Re-exports tipados dos contratos. Garante que `apps/web` falhe no build
// se a spec OpenAPI mudar de forma incompatível.

import type { Identity } from '@iris-gate/contracts';

export type Merchant = Identity.Merchant;
export type MerchantStatus = Identity.MerchantStatus;
export type Problem = Identity.Problem;
