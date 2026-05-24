import Image from 'next/image';
import type { Route } from 'next';
import { LayoutDashboard, ListChecks, PlusCircle } from 'lucide-react';
import { getTranslations } from 'next-intl/server';
import { Link } from '@/i18n/routing';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { LocaleSwitcher } from '@/components/i18n/locale-switcher';
import { logoutAction } from '@/app/[locale]/login/actions';

interface SidebarProps {
  merchantName: string;
  status: string;
}

interface NavItem<T extends string> {
  href: Route<T>;
  label: string;
  icon: typeof LayoutDashboard;
}

export async function DashboardSidebar({ merchantName, status }: SidebarProps) {
  const t = await getTranslations('dashboard.sidebar');

  const navItems: ReadonlyArray<NavItem<string>> = [
    { href: '/dashboard', label: t('nav.overview'), icon: LayoutDashboard },
    {
      href: '/dashboard/payment-intents',
      label: t('nav.payment_intents'),
      icon: ListChecks,
    },
    {
      href: '/dashboard/payment-intents/new',
      label: t('nav.new_intent'),
      icon: PlusCircle,
    },
  ];

  return (
    <aside className="hidden w-64 shrink-0 flex-col border-r bg-muted/20 md:flex">
      <div className="px-6 py-6">
        <Link href="/dashboard" className="flex items-center gap-2">
          <Image
            src="/icon.png"
            alt=""
            width={100}
            height={100}
            className="size-15 object-contain"
          />
          <Image
            src="/title.png"
            alt=""
            width={100}
            height={50}
            className="size-auto object-contain"
          />
        </Link>
      </div>
      <nav className="flex flex-1 flex-col gap-1 px-3">
        {navItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="group flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-secondary/10 hover:text-foreground"
          >
            <item.icon className="size-4 transition-colors group-hover:text-secondary" />
            {item.label}
          </Link>
        ))}
      </nav>
      <div className="space-y-3 border-t p-4">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-foreground" title={merchantName}>
              {merchantName}
            </p>
            <p className="text-xs uppercase tracking-wider text-muted-foreground">{status}</p>
          </div>
          <div className="flex items-center gap-1">
            <LocaleSwitcher />
            <ThemeToggle />
          </div>
        </div>
        <form action={logoutAction}>
          <Button type="submit" variant="outline" size="sm" className="w-full">
            {t('sign_out')}
          </Button>
        </form>
      </div>
    </aside>
  );
}
