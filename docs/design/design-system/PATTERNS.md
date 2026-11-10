# IrisGate — Patterns

> Compositions of components into recurring product moments. If you find yourself solving a layout/state problem that someone else will hit next week, document it here.

---

## 1. Page scaffolds

### Marketing / landing

```tsx
<>
  <SiteHeader />
  <main className="bg-background text-foreground">
    <Hero />
    <FeatureRow />
    <Testimonial />
    <CTA />
  </main>
  <SiteFooter />
</>
```

- Single full-width `<main>` on `--background`.
- Sections separate by vertical whitespace only — no dividers between landing sections.
- Marketing surfaces may reach for `--brand-*` for editorial accents (Hero copper italic).

### Dashboard

```tsx
<div className="flex min-h-screen bg-background">
  <Sidebar />
  <main className="flex-1 px-8 py-10">
    <header className="mb-8">
      <h1 className="font-serif text-3xl text-foreground">Overview</h1>
      <p className="text-muted-foreground">Last 24 hours of activity.</p>
    </header>
    <section className="space-y-6">{/* cards */}</section>
  </main>
</div>
```

- Sidebar fixed, main scrolls.
- Page h1 in `font-serif` (not display — display is marketing only).
- One header band per page, then content stacks vertically in `space-y-6`.
- Never use `SiteHeader` inside the dashboard.

---

## 2. Money rendering

ISO-4217 currency code (`BRL`) + amount in centavos. Format via the shared util:

```tsx
import { formatMoney } from "@/lib/format";

<span className="font-mono tabular-nums text-foreground">
  {formatMoney(intent.amount_cents, intent.currency)}
</span>
```

Always:

- `font-mono` so digits line up character-width.
- `tabular-nums` so columns of money align on the decimal regardless of the digits.
- Right-align in tables.

Never inline a hard-coded "R$" — the formatter handles symbol + locale.

---

## 3. Status display

Status has two channels: **label** (text) and **color** (badge variant). The label is the source of truth; color reinforces.

| Status | Variant | Label |
|---|---|---|
| `captured` / `succeeded` | `success` | "captured" |
| `requires_capture` / `pending` | `warning` | "review" |
| `failed` / `canceled` | `destructive` | "failed" |
| `created` / `processing` | `outline` | "processing" |

```tsx
function statusVariant(s: IntentStatus): BadgeProps["variant"] {
  switch (s) {
    case "captured":          return "success";
    case "requires_capture":  return "warning";
    case "failed":
    case "canceled":          return "destructive";
    default:                  return "outline";
  }
}

<Badge variant={statusVariant(intent.status)}>{intent.status}</Badge>
```

Centralize the variant mapping in one helper near the consumer (or in `lib/status.ts` if reused in 3+ places). Don't duplicate the switch.

---

## 4. Form fields

Forms use Next.js Server Actions + `useFormState` for error surface. Anatomy:

```tsx
<form action={action} className="space-y-5">
  <div className="space-y-1.5">
    <Label htmlFor="amount_cents">Amount (centavos)</Label>
    <Input
      id="amount_cents"
      name="amount_cents"
      inputMode="numeric"
      required
      aria-invalid={!!state.fieldErrors?.amount_cents}
      aria-describedby={state.fieldErrors?.amount_cents ? "amount_cents-err" : undefined}
    />
    {state.fieldErrors?.amount_cents && (
      <p id="amount_cents-err" className="text-sm text-destructive">
        {state.fieldErrors.amount_cents}
      </p>
    )}
  </div>

  {state.error && (
    <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
      {state.error}
    </div>
  )}

  <Button type="submit">Create payment intent</Button>
</form>
```

Rules:

- **Label always**. Visible — `sr-only` only as last resort.
- Error text uses `text-destructive`. **No red border on the input** — the message carries the signal, color-alone is not accessible.
- `aria-invalid` + `aria-describedby` wire the input to its error for screen readers.
- Form-level errors in a `role="alert"` block above the submit.
- Helper text uses `text-muted-foreground text-sm`.

---

## 5. Empty states

A card with a calm headline, one sentence of context, and a single CTA. No illustrations until the design system gets an empty-state illustration set.

```tsx
<Card>
  <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
    <h3 className="font-serif text-xl text-foreground">No payment intents yet</h3>
    <p className="text-sm text-muted-foreground max-w-prose">
      Create your first payment intent to start collecting authorizations and captures.
    </p>
    <Button asChild>
      <Link href="/dashboard/payment-intents/new">Create payment intent</Link>
    </Button>
  </CardContent>
</Card>
```

---

## 6. Loading states

Three tiers depending on what's loading:

1. **Initial route load** — Next.js `loading.tsx` at the route segment. Render the page scaffold + skeleton cards.
2. **Tabular data refresh** — keep the previous data on screen, show a small `Loader2 className="animate-spin"` in the toolbar (lucide). Don't blank the table.
3. **Submit pending** — disable the submit button, replace label with `Loader2 className="mr-2 size-4 animate-spin"` + "Creating…". Use `useFormStatus()` from `react-dom`.

```tsx
function SubmitButton() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" disabled={pending}>
      {pending ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
      {pending ? "Creating…" : "Create payment intent"}
    </Button>
  );
}
```

Skeleton component does not exist yet. When you need one, add it as `components/ui/skeleton.tsx` (shadcn `add skeleton`), wire it to `bg-muted animate-pulse`, document it in `COMPONENTS.md`.

---

## 7. Dialog vs Popover vs Drawer

| Use | When |
|---|---|
| **Dialog** (modal) | Destructive confirms ("Delete merchant"), full-form flows that block the page (rare — prefer a dedicated route) |
| **Popover** | Light-weight floating UI anchored to a trigger — date picker, filter, "more actions" menu |
| **Drawer** (mobile) | Mobile filter / nav. Not used on desktop |

Neither Dialog nor Popover ships in the components folder today. When you add the first one, follow shadcn's Radix-based primitive and ensure:

- Focus is trapped inside on open.
- `Escape` closes.
- Restore focus to the trigger on close.
- `aria-labelledby` on the dialog → the title's `id`.
- `role="dialog"` (Radix handles this).

---

## 8. Auth / login

Single-card layout, centered, on `--background`. Form uses the §4 pattern. Server Action validates the API key against the backend, sets the session cookie, redirects to `/dashboard`. Error surface is form-level (`state.error`) — most failures are "key invalid" or "network".

---

## 9. Navigation cues

- Active nav item: `text-foreground` + a 2px `border-l-2 border-primary` on the left edge (sidebar) or `border-b-2 border-primary -mb-px` (header tabs).
- Hover: `text-foreground` from `text-muted-foreground` rest state. No background-color hover on text links inside dense navigation — keep it text-only.

---

## 10. Confirmation patterns

For destructive actions (delete, cancel intent):

- **One-click destructive** is never allowed. Always: `Button variant="destructive"` → Dialog → second confirm Button inside the Dialog.
- The Dialog's confirm Button repeats the verb ("Delete merchant", not "Confirm").
- Cancel Button is `variant="outline"` next to the confirm.
- The Dialog title states the consequence ("This will permanently delete the merchant and all associated keys").

For non-destructive but irreversible (capture a payment):

- Inline confirm via the same button (e.g., "Capture R$ 1.990") with a short Popover that previews the amount + currency before the actual mutation. Single click → preview → confirm.
