import { redirect } from 'next/navigation';
import type { ReactNode } from 'react';
import { DashboardSidebar } from '@/components/dashboard/sidebar';
import { QueryProvider } from '@/components/providers/query-provider';
import { getSession } from '@/lib/session';

export const dynamic = 'force-dynamic';

export default async function DashboardLayout({ children }: { children: ReactNode }) {
  const session = await getSession();
  if (!session.apiKey || !session.merchant) {
    redirect('/login');
  }

  return (
    <QueryProvider>
      <div className="flex min-h-screen">
        <DashboardSidebar
          merchantName={session.merchant.legalName}
          status={session.merchant.status}
        />
        <main className="flex-1 overflow-x-hidden">{children}</main>
      </div>
    </QueryProvider>
  );
}
