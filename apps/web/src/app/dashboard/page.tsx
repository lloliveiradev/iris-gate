import Link from 'next/link';
import { ArrowRight, Activity, ShieldCheck, Workflow } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { authenticatedClient } from '@/lib/api-client';
import { formatDateTime, formatMoney, statusVariant } from '@/lib/format';
import { requireSession } from '@/lib/session';

export default async function DashboardOverview() {
  const session = await requireSession();
  const client = await authenticatedClient();
  const recent = await client.listPaymentIntents().catch(() => []);

  const totalCents = recent.reduce((sum, p) => sum + p.amount.amount_cents, 0);
  const captured = recent.filter((p) => p.status === 'captured').length;
  const flagged = recent.filter((p) => p.status === 'failed' || p.status === 'canceled').length;
  const currency = recent[0]?.amount.currency ?? 'BRL';

  return (
    <div className="container max-w-6xl space-y-8 px-6 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-sm text-muted-foreground">
            Bem-vindo, <strong>{session.merchant.legalName}</strong>
          </p>
          <h1 className="text-3xl font-semibold tracking-tight">Visão geral</h1>
        </div>
        <Button asChild>
          <Link href="/dashboard/payment-intents/new">
            Nova intent
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </header>

      <section className="grid gap-4 md:grid-cols-3">
        <StatCard
          icon={<Workflow className="size-4 text-primary" />}
          label="Intents recentes"
          value={String(recent.length)}
          hint="últimas 50"
        />
        <StatCard
          icon={<Activity className="size-4 text-primary" />}
          label="Volume autorizado/captado"
          value={formatMoney(totalCents, currency)}
          hint={`${captured} capturadas`}
        />
        <StatCard
          icon={<ShieldCheck className="size-4 text-primary" />}
          label="Eventos suspeitos"
          value={String(flagged)}
          hint="failed + canceled"
        />
      </section>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>Últimas intents</CardTitle>
              <CardDescription>Atualizado em tempo real ao recarregar.</CardDescription>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link href="/dashboard/payment-intents">Ver todas</Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent className="px-0 pb-0">
          {recent.length === 0 ? (
            <EmptyState />
          ) : (
            <ul className="divide-y border-t">
              {recent.slice(0, 6).map((p) => (
                <li key={p.id} className="flex items-center justify-between gap-4 px-6 py-4">
                  <div className="min-w-0">
                    <Link
                      href={`/dashboard/payment-intents/${p.id}`}
                      className="block truncate font-medium hover:underline"
                    >
                      {formatMoney(p.amount.amount_cents, p.amount.currency)}
                    </Link>
                    <p className="truncate text-xs text-muted-foreground">
                      {p.id} · {formatDateTime(p.created_at)}
                    </p>
                  </div>
                  <Badge variant={statusVariant(p.status)} className="shrink-0">
                    {p.status}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  hint,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2 pb-2">
        <CardDescription className="text-sm">{label}</CardDescription>
        <span className="grid size-8 place-items-center rounded-md bg-muted">{icon}</span>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-semibold tracking-tight">{value}</p>
        {hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}
      </CardContent>
    </Card>
  );
}

function EmptyState() {
  return (
    <div className="border-t p-12 text-center">
      <p className="text-sm text-muted-foreground">
        Nenhum payment intent ainda. Crie o primeiro pra ver os dados aqui.
      </p>
      <Button asChild className="mt-4">
        <Link href="/dashboard/payment-intents/new">Criar payment intent</Link>
      </Button>
    </div>
  );
}
