import Image from 'next/image';
import { getTranslations } from 'next-intl/server';

export async function SiteFooter() {
  const t = await getTranslations('site.footer');

  const productItems = [
    t('columns.product.items.payments'),
    t('columns.product.items.anti_fraud'),
    t('columns.product.items.reporting'),
    t('columns.product.items.webhooks'),
  ];
  const devItems = [
    t('columns.developers.items.api_reference'),
    t('columns.developers.items.status'),
    t('columns.developers.items.changelog'),
  ];
  const companyItems = [
    t('columns.company.items.about'),
    t('columns.company.items.security'),
    t('columns.company.items.contact'),
  ];

  return (
    <footer className="border-t border-border/40">
      <div className="container mx-auto grid gap-8 px-6 py-12 md:grid-cols-4">
        <div>
          <div className="flex items-center gap-2.5 text-base font-semibold">
            <Image
              src="/icon.png"
              alt=""
              width={56}
              height={56}
              className="size-7 rounded object-contain"
            />
            <span>IrisGate</span>
          </div>
          <p className="mt-3 text-sm text-muted-foreground">{t('tagline')}</p>
        </div>
        <FooterCol title={t('columns.product.title')} items={productItems} />
        <FooterCol title={t('columns.developers.title')} items={devItems} />
        <FooterCol title={t('columns.company.title')} items={companyItems} />
      </div>
      <div className="border-t border-border/40">
        <div className="container mx-auto flex flex-col gap-2 px-6 py-6 text-xs text-muted-foreground md:flex-row md:items-center md:justify-between">
          <p>{t('copyright', { year: new Date().getFullYear() })}</p>
          <p>{t('built_for')}</p>
        </div>
      </div>
    </footer>
  );
}

function FooterCol({ title, items }: { title: string; items: string[] }) {
  return (
    <div>
      <p className="text-sm font-semibold">{title}</p>
      <ul className="mt-3 space-y-2 text-sm text-muted-foreground">
        {items.map((it) => (
          <li key={it}>
            <span className="hover:text-foreground">{it}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
