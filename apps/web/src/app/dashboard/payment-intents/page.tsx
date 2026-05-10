import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { authenticatedClient } from '@/lib/api-client';
import { formatDateTime, formatMoney, statusVariant } from '@/lib/format';

export default async function PaymentIntentsList() {
  const client = await authenticatedClient();
  const intents = await client.listPaymentIntents();

  return (
    <div className="container max-w-6xl space-y-6 px-6 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Payment intents</h1>
          <p className="text-sm text-muted-foreground">
            Listagem das últimas {intents.length} intents do merchant.
          </p>
        </div>
        <Button asChild>
          <Link href="/dashboard/payment-intents/new">
            Nova intent
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Histórico</CardTitle>
          <CardDescription>Mais recentes primeiro · clique numa linha para detalhes.</CardDescription>
        </CardHeader>
        <CardContent className="px-0 pb-0">
          {intents.length === 0 ? (
            <div className="border-t p-12 text-center text-sm text-muted-foreground">
              Nenhuma intent ainda.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-6">ID</TableHead>
                  <TableHead>Valor</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Criado</TableHead>
                  <TableHead className="pr-6 text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {intents.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="pl-6 font-mono text-xs">{p.id}</TableCell>
                    <TableCell className="font-medium">
                      {formatMoney(p.amount.amount_cents, p.amount.currency)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(p.status)}>{p.status}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDateTime(p.created_at)}
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      <Button asChild size="sm" variant="ghost">
                        <Link href={`/dashboard/payment-intents/${p.id}`}>Detalhes</Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
