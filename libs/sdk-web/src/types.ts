import type { Identity, Payments } from '@iris-gate/contracts';

export type Merchant = Identity.Merchant;
export type MerchantStatus = Identity.MerchantStatus;

export type Money = Payments.Money;
export type CreatePaymentIntentRequest = Payments.CreatePaymentIntentRequest;
export type PaymentIntent = Payments.PaymentIntent;
export type PaymentIntentStatus = Payments.PaymentIntentStatus;

export type Problem = Identity.Problem | Payments.Problem;
