import Link from 'next/link';
import { ChevronLeft } from 'lucide-react';
import type { Metadata } from 'next';
import { getTranslations } from 'next-intl/server';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { NewIntentForm } from './new-intent-form';

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations('dashboard.payment_intents_new');
  return { title: t('metadata_title') };
}

export default async function NewPaymentIntentPage() {
  const t = await getTranslations('dashboard.payment_intents_new');

  return (
    <div className="container max-w-2xl space-y-6 px-6 py-8">
      <Button asChild variant="ghost" size="sm" className="w-fit">
        <Link href="/dashboard/payment-intents">
          <ChevronLeft className="size-4" />
          {t('back')}
        </Link>
      </Button>

      <header className="space-y-1">
        <p className="text-sm font-medium uppercase tracking-wider text-secondary">
          {t('eyebrow')}
        </p>
        <h1 className="font-serif text-3xl font-normal tracking-tight text-foreground">
          {t('title')}
        </h1>
        <p className="text-sm text-muted-foreground">{t('subtitle')}</p>
      </header>
      <Card>
        <CardHeader>
          <CardTitle>{t('card_title')}</CardTitle>
          <CardDescription>{t('card_description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <NewIntentForm />
        </CardContent>
      </Card>
    </div>
  );
}
