import Link from 'next/link';
import { redirect } from 'next/navigation';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { getSession } from '@/lib/session';
import { LoginForm } from './login-form';

export const dynamic = 'force-dynamic';

export default async function LoginPage() {
  const session = await getSession();
  if (session.apiKey) redirect('/dashboard');

  return (
    <div className="flex min-h-screen flex-col bg-muted/40">
      <header className="container mx-auto px-6 py-6">
        <Link href="/" className="flex items-center gap-2 text-sm font-semibold">
          <span className="grid size-7 place-items-center rounded bg-primary text-primary-foreground text-xs">
            ⬢
          </span>
          IrisGate
        </Link>
      </header>
      <main className="flex flex-1 items-center justify-center p-6">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle className="text-2xl">Entrar no painel</CardTitle>
            <CardDescription>
              Use a API key emitida em <code className="font-mono">/v1/api-keys</code>.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <LoginForm />
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
