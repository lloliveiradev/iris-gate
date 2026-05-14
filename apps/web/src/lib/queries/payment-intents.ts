'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { PaymentIntent } from '@iris-gate/sdk-web';
import {
  authorizePaymentIntentAction,
  capturePaymentIntentAction,
  listPaymentIntentsAction,
} from '@/app/dashboard/payment-intents/queries';

export const paymentIntentKeys = {
  all: ['payment-intents'] as const,
  list: () => [...paymentIntentKeys.all, 'list'] as const,
  detail: (id: string) => [...paymentIntentKeys.all, 'detail', id] as const,
};

export function useListPaymentIntents(options?: { initialData?: PaymentIntent[] }) {
  return useQuery({
    queryKey: paymentIntentKeys.list(),
    queryFn: () => listPaymentIntentsAction(),
    staleTime: 30_000,
    initialData: options?.initialData,
  });
}

export function useAuthorizePaymentIntent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: authorizePaymentIntentAction,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: paymentIntentKeys.all });
    },
  });
}

export function useCapturePaymentIntent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: capturePaymentIntentAction,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: paymentIntentKeys.all });
    },
  });
}
