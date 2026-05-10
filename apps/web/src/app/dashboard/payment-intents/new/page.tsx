import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { NewIntentForm } from './new-intent-form';

export const metadata = { title: 'Nova payment intent' };

export default function NewPaymentIntentPage() {
  return (
    <div className="container max-w-2xl space-y-6 px-6 py-8">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight">Nova payment intent</h1>
        <p className="text-sm text-muted-foreground">
          A chave de idempotência é gerada automaticamente — submeter duas vezes não duplica a
          intent.
        </p>
      </header>
      <Card>
        <CardHeader>
          <CardTitle>Detalhes</CardTitle>
          <CardDescription>Todos os campos são obrigatórios exceto descrição.</CardDescription>
        </CardHeader>
        <CardContent>
          <NewIntentForm />
        </CardContent>
      </Card>
    </div>
  );
}
