import type { Identity, Payments } from '@iris-gate/contracts';

export type Merchant = Identity.Merchant;
export type CreateMerchantRequest = Identity.CreateMerchantRequest;
export type MerchantStatus = Identity.MerchantStatus;
export type ApiKey = Identity.ApiKey;
export type IssueApiKeyRequest = Identity.IssueApiKeyRequest;
export type IssueApiKeyResponse = Identity.IssueApiKeyResponse;

export type Money = Payments.Money;
export type CreatePaymentIntentRequest = Payments.CreatePaymentIntentRequest;
export type PaymentIntent = Payments.PaymentIntent;
export type PaymentIntentStatus = Payments.PaymentIntentStatus;

export type Problem = Identity.Problem | Payments.Problem;
