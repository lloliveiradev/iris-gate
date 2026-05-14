'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import {
  Check,
  CircleDollarSign,
  Copy,
  Eye,
  Loader2,
  ShieldCheck,
} from 'lucide-react';
import type { PaymentIntent } from '@iris-gate/sdk-web';
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu';
import { DataTableRowActions } from '@/components/data-table/row-actions';
import {
  useAuthorizePaymentIntent,
  useCapturePaymentIntent,
} from '@/lib/queries/payment-intents';

interface PaymentIntentRowActionsProps {
  intent: PaymentIntent;
}

export function PaymentIntentRowActions({ intent }: PaymentIntentRowActionsProps) {
  const authorize = useAuthorizePaymentIntent();
  const capture = useCapturePaymentIntent();
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (copyTimer.current) clearTimeout(copyTimer.current);
    };
  }, []);

  const canAuthorize = intent.status === 'pending';
  const canCapture = intent.status === 'authorized';
  const authorizing = authorize.isPending && authorize.variables?.id === intent.id;
  const capturing = capture.isPending && capture.variables?.id === intent.id;

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(intent.id);
      setCopied(true);
      if (copyTimer.current) clearTimeout(copyTimer.current);
      copyTimer.current = setTimeout(() => setCopied(false), 1500);
    } catch {
      // navigator.clipboard may reject in non-secure contexts; silently ignore.
    }
  };

  return (
    <DataTableRowActions label="Ações da intent">
      <DropdownMenuItem asChild>
        <Link href={`/dashboard/payment-intents/${intent.id}`} className="gap-2">
          <Eye className="size-4" aria-hidden />
          Ver detalhes
        </Link>
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem
        disabled={!canAuthorize || authorizing}
        onSelect={(event) => {
          event.preventDefault();
          if (!canAuthorize) return;
          authorize.mutate({ id: intent.id });
        }}
        className="gap-2"
      >
        {authorizing ? (
          <Loader2 className="size-4 animate-spin" aria-hidden />
        ) : (
          <ShieldCheck className="size-4" aria-hidden />
        )}
        Autorizar
      </DropdownMenuItem>
      <DropdownMenuItem
        disabled={!canCapture || capturing}
        onSelect={(event) => {
          event.preventDefault();
          if (!canCapture) return;
          capture.mutate({ id: intent.id });
        }}
        className="gap-2"
      >
        {capturing ? (
          <Loader2 className="size-4 animate-spin" aria-hidden />
        ) : (
          <CircleDollarSign className="size-4" aria-hidden />
        )}
        Capturar
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem
        onSelect={(event) => {
          event.preventDefault();
          void onCopy();
        }}
        className="gap-2"
      >
        {copied ? (
          <Check className="size-4 text-emerald-500" aria-hidden />
        ) : (
          <Copy className="size-4" aria-hidden />
        )}
        {copied ? 'Copiado' : 'Copiar ID'}
      </DropdownMenuItem>
    </DataTableRowActions>
  );
}
