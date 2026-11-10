# IrisGate — Components

> Inventory of UI components shipping today, their variants, the tokens they consume, and how to use them. New component? Add it here in the same shape, alphabetically within its section.

Primitives live in `apps/web/src/components/ui/` — they wrap shadcn/ui new-york. Custom app components live in domain-scoped folders (`site/`, `landing/`, `dashboard/`, `theme/`, `providers/`).

---

## Primitives (shadcn-derived)

### Badge — `components/ui/badge.tsx`

Compact status / metadata pill.

**Variants:** `default` | `secondary` | `outline` | `success` | `warning` | `destructive`

| Variant | Use |
|---|---|
| `default` | Generic emphasis (rare — prefer the others) |
| `outline` | Neutral meta (e.g., "Now in private beta") |
| `success` | "captured", "approved", "active" |
| `warning` | "review", "pending review" |
| `destructive` | "failed", "declined" |

```tsx
<Badge variant="success">captured</Badge>
<Badge variant="warning">review</Badge>
<Badge variant="destructive">failed</Badge>
```

Status badges should not rely on color alone — see `ACCESSIBILITY.md §3`.

### Button — `components/ui/button.tsx`

**Variants:** `default` | `destructive` | `outline` | `secondary` | `ghost` | `link`
**Sizes:** `default` | `sm` | `lg` | `icon`

| Variant | When |
|---|---|
| `default` | Primary CTA on the page — one per view |
| `secondary` | Premium / copper CTA — "Upgrade to Pro", "Capture R$ 1.990" |
| `outline` | Co-equal alternative ("Cancel", "Back") |
| `ghost` | Toolbar / icon buttons inside dense UI (header, sidebar) |
| `link` | Inline text-styled action |
| `destructive` | Confirmed-destruction in a dialog ("Delete merchant") |

```tsx
<Button>Connect merchant</Button>
<Button variant="secondary">Upgrade to Pro</Button>
<Button variant="outline" size="sm">Cancel</Button>
<Button variant="ghost" size="icon" aria-label="Toggle theme">
  <Sun aria-hidden />
</Button>
```

One `default` Button per primary action group. Don't stack two `default`s side-by-side — pair `default` + `outline`.

### Card — `components/ui/card.tsx`

Compound: `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`.

Cards consume `--card` / `--card-foreground`. They sit on `--background`; the lightness step-up provides elevation. Avoid stacking cards inside cards — use `Separator` instead.

```tsx
<Card>
  <CardHeader>
    <CardTitle>Recent payment intents</CardTitle>
    <CardDescription>Last 24 hours</CardDescription>
  </CardHeader>
  <CardContent>{/* table */}</CardContent>
  <CardFooter>
    <Button variant="link">View all</Button>
  </CardFooter>
</Card>
```

`CardTitle` defaults to `font-serif` weight via Tailwind utility classes — keep that. Don't override with `font-display` (that's hero-only).

### Input — `components/ui/input.tsx`

Plain HTML `<input>`, forwardRef'd, themed via `--input` and `--ring`. Pair with `Label`. No `helperText` prop — render the helper text yourself under the input.

```tsx
<div className="space-y-1.5">
  <Label htmlFor="amount">Amount</Label>
  <Input id="amount" name="amount_cents" inputMode="numeric" />
  <p className="text-sm text-muted-foreground">In centavos. BRL only for now.</p>
</div>
```

Error state: render an error message in `text-destructive text-sm` below the input. Do not add a red border — color-alone is not enough; the message carries the signal.

### Label — `components/ui/label.tsx`

Radix Label. `text-sm font-medium` by default. Always pair with an input via `htmlFor`.

### Separator — `components/ui/separator.tsx`

Radix Separator with `role="separator"`. Use between sections in a card, in dropdown menus, in form groups.

### Table — `components/ui/table.tsx`

Compound: `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell`. Header row sits at `--surface` lightness; body rows at `--card`. Money columns: `font-mono tabular-nums text-right`.

```tsx
<Table>
  <TableHeader>
    <TableRow>
      <TableHead>ID</TableHead>
      <TableHead className="text-right">Amount</TableHead>
      <TableHead>Status</TableHead>
    </TableRow>
  </TableHeader>
  <TableBody>
    {intents.map((p) => (
      <TableRow key={p.id}>
        <TableCell className="font-mono text-xs">{p.id}</TableCell>
        <TableCell className="font-mono tabular-nums text-right">
          {formatMoney(p.amount_cents, p.currency)}
        </TableCell>
        <TableCell>
          <Badge variant={statusVariant(p.status)}>{p.status}</Badge>
        </TableCell>
      </TableRow>
    ))}
  </TableBody>
</Table>
```

---

## App components

### SiteHeader — `components/site/site-header.tsx`

Sticky top bar (`h-16`, `z-40`, `bg-background/80 backdrop-blur`). Logo + primary nav + `ThemeToggle` + auth buttons. Renders on marketing routes; do not reuse inside the dashboard (the dashboard has its own chrome via `Sidebar`).

### SiteFooter — `components/site/site-footer.tsx`

4-column grid (product / dev / company / legal). `bg-surface text-muted-foreground` with `--foreground` headings. Marketing-only.

### Hero — `components/landing/hero.tsx`

Two-column above the fold. Left: badge + h1 + lede + CTA pair. Right: product still / illustration.

The h1 uses three font treatments in one heading: `font-display italic` for the main line, `font-serif font-normal` for a secondary phrase, `text-brand-copper italic` for the editorial flourish word. This is **the** place where reaching for `text-brand-copper` is correct — see `PRINCIPLES.md §4` and §8.

### CTA — `components/landing/cta.tsx`

Single section block, ~1.4:1 grid (image left, copy + button right). Wrapped in a `rounded-3xl` card on `--surface`. Use at the bottom of the landing page only.

### Testimonial — `components/landing/testimonial.tsx`

Card with a subtle gradient overlay (`bg-gradient-to-br from-primary/5`). Blockquote + author + a 3-stat grid below. One per page max.

### FeatureRow — `components/landing/feature-row.tsx`

Three-icon feature triplet for the landing page. Icons via Lucide (`Activity`, `ShieldCheck`, `Workflow`).

### Sidebar — `components/dashboard/sidebar.tsx`

Fixed left aside, `w-64`, `hidden md:flex`, `bg-muted/20`. Nav items: icon + label, `text-muted-foreground` at rest, `text-foreground` on hover/active. Logout at the bottom as `Button variant="outline" size="sm"`.

Section headers above nav groups: `text-xs uppercase tracking-wider text-muted-foreground`.

### ThemeToggle — `components/theme/theme-toggle.tsx`

`Button variant="ghost" size="icon"`. Renders `Sun` in dark mode, `Moon` in light mode (lucide-react), both `aria-hidden`. The button itself carries a context-aware `aria-label` — "Switch to light theme" / "Switch to dark theme". Persists choice to `localStorage.iris-theme`.

State is initialised as `null` and hydrated on first `useEffect` to dodge SSR mismatch — see the file for the pattern; do not replicate via SSR-rendered state.

### ThemeScript — `components/theme/theme-script.tsx`

`<script>` injected into `<head>` of `RootLayout` via `dangerouslySetInnerHTML`. Reads `localStorage.iris-theme` and applies the class on `<html>` **before** React hydrates, preventing FOUC. React will warn — the warning is expected; this is the canonical pattern.

### QueryProvider — `components/providers/query-provider.tsx`

React Query wrapper for the dashboard data layer. Wraps children in `<QueryClientProvider>`. Mount inside `RootLayout` (or the dashboard layout if you want to limit scope).

---

## Adding a new component

1. Decide: primitive (goes in `components/ui/`) or app component (goes in a domain folder).
2. If primitive, prefer shadcn — `npx shadcn@latest add <name>`, then adjust the variants to use semantic tokens, not stock shadcn theme vars.
3. If app component, follow the existing folder conventions (`landing/`, `dashboard/`, `site/`).
4. **Wire only semantic tokens** in product components (`bg-card`, `text-foreground`, `text-muted-foreground`, `bg-primary`, `bg-secondary`, etc.). Reaching for `--brand-*` is a marketing-only escape hatch.
5. Add an entry to this doc with: file path, variants (if any), a real `<JSX>` usage example, and any do/don'ts.
6. If the component introduces a new pattern (status display, money rendering, empty state), document it in `PATTERNS.md` too.
