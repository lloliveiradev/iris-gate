import { IrisApiError } from './errors';
import type {
  CreateMerchantRequest,
  CreatePaymentIntentRequest,
  IssueApiKeyRequest,
  IssueApiKeyResponse,
  Merchant,
  PaymentIntent,
} from './types';

export interface IrisClientOptions {
  baseUrl: string;
  apiKey?: string;
  fetch?: typeof fetch;
}

export class IrisClient {
  private readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(opts: IrisClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, '');
    this.apiKey = opts.apiKey;
    this.fetchImpl = opts.fetch ?? fetch;
  }

  withApiKey(apiKey: string): IrisClient {
    return new IrisClient({ baseUrl: this.baseUrl, apiKey, fetch: this.fetchImpl });
  }

  async createMerchant(input: CreateMerchantRequest): Promise<Merchant> {
    return this.request<Merchant>('POST', '/v1/merchants', input);
  }

  async me(): Promise<Merchant> {
    return this.request<Merchant>('GET', '/v1/me');
  }

  async issueApiKey(input: IssueApiKeyRequest): Promise<IssueApiKeyResponse> {
    return this.request<IssueApiKeyResponse>('POST', '/v1/api-keys', input);
  }

  async revokeApiKey(id: string): Promise<void> {
    await this.request<unknown>('POST', `/v1/api-keys/${encodeURIComponent(id)}/revoke`);
  }

  async createPaymentIntent(
    input: CreatePaymentIntentRequest,
    idempotencyKey: string,
  ): Promise<PaymentIntent> {
    return this.request<PaymentIntent>('POST', '/v1/payment-intents', input, {
      headers: { 'Idempotency-Key': idempotencyKey },
    });
  }

  async authorizePaymentIntent(id: string): Promise<PaymentIntent> {
    return this.request<PaymentIntent>(
      'POST',
      `/v1/payment-intents/${encodeURIComponent(id)}/authorize`,
    );
  }

  async capturePaymentIntent(id: string): Promise<PaymentIntent> {
    return this.request<PaymentIntent>(
      'POST',
      `/v1/payment-intents/${encodeURIComponent(id)}/capture`,
    );
  }

  async getPaymentIntent(id: string): Promise<PaymentIntent> {
    return this.request<PaymentIntent>('GET', `/v1/payment-intents/${encodeURIComponent(id)}`);
  }

  async listPaymentIntents(): Promise<PaymentIntent[]> {
    return this.request<PaymentIntent[]>('GET', '/v1/payment-intents');
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    init: { headers?: Record<string, string> } = {},
  ): Promise<T> {
    const headers: Record<string, string> = {
      Accept: 'application/json',
      ...(init.headers ?? {}),
    };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;

    const res = await this.fetchImpl(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      cache: 'no-store',
    });

    if (!res.ok) {
      throw await IrisApiError.fromResponse(res);
    }

    if (res.status === 204) return undefined as T;
    const ct = res.headers.get('content-type') ?? '';
    if (!ct.includes('application/json')) return undefined as T;
    return (await res.json()) as T;
  }
}
