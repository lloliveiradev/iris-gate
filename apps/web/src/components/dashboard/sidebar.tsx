import Link from 'next/link';
import type { Route } from 'next';
import { LayoutDashboard, ListChecks, PlusCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { logoutAction } from '@/app/login/actions';

interface SidebarProps {
  merchantName: string;
  status: string;
}

interface NavItem<T extends string> {
  href: Route<T>;
  label: string;
  icon: typeof LayoutDashboard;
}

const navItems: ReadonlyArray<NavItem<string>> = [
  { href: '/dashboard', label: 'Overview', icon: LayoutDashboard },
  { href: '/dashboard/payment-intents', label: 'Payment intents', icon: ListChecks },
  { href: '/dashboard/payment-intents/new', label: 'New intent', icon: PlusCircle },
];

export function DashboardSidebar({ merchantName, status }: SidebarProps) {
  return (
    <aside className="hidden w-64 shrink-0 flex-col border-r bg-muted/20 md:flex">
      <div className="px-6 py-6">
        <Link href="/dashboard" className="flex items-center gap-2 text-base font-semibold">
          <span className="grid size-8 place-items-center rounded bg-primary text-primary-foreground text-xs">
            ⬢
          </span>
          IrisGate
        </Link>
      </div>
      <nav className="flex flex-1 flex-col gap-1 px-3">
        {navItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <item.icon className="size-4" />
            {item.label}
          </Link>
        ))}
      </nav>
      <div className="border-t p-4">
        <div className="mb-3">
          <p className="truncate text-sm font-medium" title={merchantName}>
            {merchantName}
          </p>
          <p className="text-xs uppercase tracking-wider text-muted-foreground">{status}</p>
        </div>
        <form action={logoutAction}>
          <Button type="submit" variant="outline" size="sm" className="w-full">
            Sign out
          </Button>
        </form>
      </div>
    </aside>
  );
}
