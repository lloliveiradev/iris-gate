export function SiteFooter() {
  return (
    <footer className="border-t border-border/40">
      <div className="container mx-auto grid gap-8 px-6 py-12 md:grid-cols-4">
        <div>
          <div className="flex items-center gap-2 text-base font-semibold">
            <span className="grid size-7 place-items-center rounded bg-primary text-primary-foreground text-xs">
              ⬢
            </span>
            IrisGate
          </div>
          <p className="mt-3 text-sm text-muted-foreground">
            Contract-first payment infrastructure for Brazilian merchants.
          </p>
        </div>
        <FooterCol
          title="Product"
          items={['Payments', 'Anti-fraud', 'Reporting', 'Webhooks']}
        />
        <FooterCol title="Developers" items={['API reference', 'Status', 'Changelog']} />
        <FooterCol title="Company" items={['About', 'Security', 'Contact']} />
      </div>
      <div className="border-t border-border/40">
        <div className="container mx-auto flex flex-col gap-2 px-6 py-6 text-xs text-muted-foreground md:flex-row md:items-center md:justify-between">
          <p>© {new Date().getFullYear()} IrisGate.</p>
          <p>Built for the Phase 5 milestone of the iris-gate roadmap.</p>
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
