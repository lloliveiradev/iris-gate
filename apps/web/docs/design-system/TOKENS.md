# IrisGate — Design Tokens

> **Stack:** Tailwind 4 (OKLCH-native) + shadcn/ui new-york + CSS variables.
> Runtime artifact: `apps/web/src/app/globals.css`.

This is the single source of truth for color, typography, spacing, radius, shadow, and motion tokens. Every value below ships as a CSS variable. Components consume the **semantic** layer (`--primary`, `--card`, …) — never the **brand** layer (`--brand-copper`) directly, unless intentionally bypassing themability for a marketing surface.

---

## 1. Brand palette (primitives)

The 6 brand colors the founder picked, plus 2 derived neutrals to make a 3-tier dark + 3-tier light surface system.

| Token                            | HEX       | OKLCH                   | Role hint                              |
| -------------------------------- | --------- | ----------------------- | -------------------------------------- |
| `--brand-ink`                    | `#081B1B` | `oklch(0.18 0.012 195)` | Deepest petrol — dark base bg          |
| `--brand-petrol`                 | `#203B37` | `oklch(0.32 0.022 174)` | Dark teal — elevated surfaces          |
| `--brand-sage`                   | `#5A8F76` | `oklch(0.60 0.053 156)` | Primary brand — CTAs, links            |
| `--brand-mint`                   | `#96CDB0` | `oklch(0.79 0.063 158)` | Accent — hover, focus, success-soft    |
| `--brand-copper`                 | `#C18D52` | `oklch(0.67 0.105 65)`  | Secondary brand — premium CTAs, alerts |
| `--brand-cream`                  | `#EEE8B2` | `oklch(0.92 0.058 102)` | Dark-mode body text; marketing accent  |
| `--brand-petrol-mid` *(derived)* | `#142E2A` | `oklch(0.25 0.018 178)` | Mid layer for 3-tier dark surfaces     |
| `--brand-cream-soft` *(derived)* | `#F5EFC9` | `oklch(0.95 0.04 100)`  | Marketing accent (paired with cream)   |

**Personality:** "artisanal fintech" — petrol + sage as the trust backbone, copper used assertively (CTAs and alerts), cream as the inversion. Anti-corporate-blue.

---

## 2. Semantic layer — Dark theme (default)

Dark is the **default theme**. The `<html>` element ships with `class="dark"` set in `layout.tsx`. Opt out by removing the class or applying `.light`.

### Surfaces (3 levels)

| Variable               | Value                            | Used by                                  | Rationale                    |
| ---------------------- | -------------------------------- | ---------------------------------------- | ---------------------------- |
| `--background`         | `--brand-ink` (`#081B1B`)        | `<body>`, scaffolds                      | L1 — deepest, page bg        |
| `--surface`            | `--brand-petrol-mid` (`#142E2A`) | Sidebar, secondary panels, table headers | L2 — quieter elevation       |
| `--card` / `--popover` | `--brand-petrol` (`#203B37`)     | Cards, dialogs, dropdowns                | L3 — content lifted above bg |

### Text

| Variable               | Value                                                 | Use                                     |
| ---------------------- | ----------------------------------------------------- | --------------------------------------- |
| `--foreground`         | `--brand-cream` (`#EEE8B2`)                           | Body text, primary copy                 |
| `--muted-foreground`   | `--brand-mint` at 70% (`oklch(0.79 0.063 158 / 70%)`) | Secondary text, helper text, timestamps |
| `--card-foreground`    | `--brand-cream`                                       | Text on cards (same as body)            |
| `--popover-foreground` | `--brand-cream`                                       | Text in popovers/dropdowns              |

### Brand actions

| Variable                 | Value                            | Use                                               |
| ------------------------ | -------------------------------- | ------------------------------------------------- |
| `--primary`              | `--brand-sage` (`#5A8F76`)       | Default CTAs, link emphasis                       |
| `--primary-foreground`   | `--brand-ink`                    | Text on primary surfaces                          |
| `--secondary`            | `--brand-copper` (`#C18D52`)     | Premium CTAs, "upgrade" buttons, important alerts |
| `--secondary-foreground` | `--brand-ink`                    | Text on copper                                    |
| `--accent`               | `--brand-mint` (`#96CDB0`)       | Hover bgs, soft highlights, success-soft          |
| `--accent-foreground`    | `--brand-ink`                    | Text on mint                                      |
| `--muted`                | `--brand-petrol-mid` (`#142E2A`) | Subtle bg fills (disabled, skeletons)             |

### Borders + inputs + focus

| Variable   | Value                                      | Use                               |
| ---------- | ------------------------------------------ | --------------------------------- |
| `--border` | `oklch(0.60 0.053 156 / 22%)` (sage @ 22%) | Card borders, dividers            |
| `--input`  | `oklch(0.60 0.053 156 / 28%)`              | Input borders (slightly stronger) |
| `--ring`   | `--brand-mint`                             | Focus ring — high visibility      |

### Feedback (derived to harmonize with palette)

| Variable                   | OKLCH                  | Notes                                                               |
| -------------------------- | ---------------------- | ------------------------------------------------------------------- |
| `--destructive`            | `oklch(0.58 0.18 25)`  | Brick red — warmer than stock red, matches cream/copper temperature |
| `--destructive-foreground` | `--brand-cream`        | Cream text on red                                                   |
| `--success`                | `--brand-mint`         | Mint doubles as "approved", "captured", "success"                   |
| `--warning`                | `--brand-copper`       | Copper doubles as "review", "attention"                             |
| `--info`                   | `oklch(0.63 0.10 220)` | Muted teal-blue — calibrated to palette saturation                  |

### Chart palette (5-tone, harmonized)

For data viz / Recharts / observability dashboards.

| Variable    | OKLCH                   | Hue family          |
| ----------- | ----------------------- | ------------------- |
| `--chart-1` | `oklch(0.60 0.053 156)` | sage                |
| `--chart-2` | `oklch(0.67 0.105 65)`  | copper              |
| `--chart-3` | `oklch(0.79 0.063 158)` | mint                |
| `--chart-4` | `oklch(0.92 0.058 102)` | cream               |
| `--chart-5` | `oklch(0.55 0.10 200)`  | derived petrol-teal |

### Sidebar (treats sidebar as its own scope — shadcn convention)

| Variable                       | Value                                   |
| ------------------------------ | --------------------------------------- |
| `--sidebar`                    | `--surface` (`#142E2A`)                 |
| `--sidebar-foreground`         | `--brand-cream`                         |
| `--sidebar-primary`            | `--brand-sage`                          |
| `--sidebar-primary-foreground` | `--brand-ink`                           |
| `--sidebar-accent`             | `--brand-petrol` (`#203B37`) — hover bg |
| `--sidebar-accent-foreground`  | `--brand-cream`                         |
| `--sidebar-border`             | `oklch(0.60 0.053 156 / 18%)`           |
| `--sidebar-ring`               | `--brand-mint`                          |

---

## 3. Semantic layer — Light theme

Apply by setting `class="light"` on `<html>` (or scoping to a subtree).

### Surfaces (3 levels)

No pure white, no cream-as-base. Surfaces are near-white off-grays with a whisper of petrol hue (~174) at very low chroma — calm "paper-but-not-parchment" feel that pairs cleanly with copper/sage accents and won't fatigue the eyes. Cream primitives stay available for marketing accents only.

| Variable               | Value                          | Used by                                  | Rationale                                  |
| ---------------------- | ------------------------------ | ---------------------------------------- | ------------------------------------------ |
| `--background`         | `oklch(0.97 0.003 174)`        | `<body>`, scaffolds                      | L1 — neutral mist with petrol whisper      |
| `--surface`            | `oklch(0.955 0.004 174)`       | Sidebar, panels, table headers           | L2 — one step down from bg                 |
| `--card` / `--popover` | `oklch(0.985 0.003 174)`       | Cards, dialogs, dropdowns                | L3 — elevated above bg, never pure white   |

### Text

| Variable             | Value                          | Use            |
| -------------------- | ------------------------------ | -------------- |
| `--foreground`       | `--brand-ink` (`#081B1B`)      | Body           |
| `--muted-foreground` | `oklch(0.36 0.025 174)` solid  | Secondary text |

### Brand actions

| Variable                 | Value                                                                          |
| ------------------------ | ------------------------------------------------------------------------------ |
| `--primary`              | `--brand-petrol` (`#203B37`) — reads as strong primary on neutral mist         |
| `--primary-foreground`   | `oklch(0.985 0.003 174)` — same near-white as card, so CTAs are solid blocks   |
| `--secondary`            | `--brand-copper` (`#C18D52`) — same as dark                                    |
| `--secondary-foreground` | `--brand-ink`                                                                  |
| `--accent`               | `--brand-sage` at 22% (subtle mint-tint)                                       |
| `--accent-foreground`    | `--brand-petrol`                                                               |

### Borders / inputs / focus

| Variable   | Value                                        |
| ---------- | -------------------------------------------- |
| `--border` | `oklch(0.32 0.022 174 / 18%)` (petrol @ 18%) |
| `--input`  | `oklch(0.32 0.022 174 / 28%)`                |
| `--ring`   | `--brand-sage`                               |

---

## 4. Typography

### Font families (4 tiers)

| Variable         | Family                        | Source                       | Purpose                                                      |
| ---------------- | ----------------------------- | ---------------------------- | ------------------------------------------------------------ |
| `--font-sans`    | **Geist Sans**                | `geist` npm pkg, self-hosted | Body UI, buttons, inputs, labels, paragraphs                 |
| `--font-serif`   | **Fraunces** (variable)       | Google Fonts via `next/font` | Card titles, h3/h4, section headings — warm editorial weight |
| `--font-display` | **Instrument Serif** (italic) | Google Fonts via `next/font` | Hero h1, marketing display — modern calligraphic feel        |
| `--font-mono`    | **Geist Mono**                | `geist` npm pkg              | Code, monetary amounts, UUIDs, timestamps                    |

**Application rule of thumb:**
- `<h1>` hero → `font-display italic`
- `<h2>` section → `font-serif` (regular)
- `<h3>`, card titles → `font-serif` (or `font-sans semibold` if dense)
- `<h4>` and below → `font-sans` semibold
- Body, labels, buttons → `font-sans`
- Amounts (R$ 1.990,00), IDs, code → `font-mono`

### Type scale (Tailwind defaults, unchanged)

| Token       | Size            | Line height |
| ----------- | --------------- | ----------- |
| `text-xs`   | 0.75rem (12px)  | 1rem        |
| `text-sm`   | 0.875rem (14px) | 1.25rem     |
| `text-base` | 1rem (16px)     | 1.5rem      |
| `text-lg`   | 1.125rem (18px) | 1.75rem     |
| `text-xl`   | 1.25rem (20px)  | 1.75rem     |
| `text-2xl`  | 1.5rem (24px)   | 2rem        |
| `text-3xl`  | 1.875rem (30px) | 2.25rem     |
| `text-4xl`  | 2.25rem (36px)  | 2.5rem      |
| `text-5xl`  | 3rem (48px)     | 1           |
| `text-6xl`  | 3.75rem (60px)  | 1           |
| `text-7xl`  | 4.5rem (72px)   | 1           |
| `text-8xl`  | 6rem (96px)     | 1           |
| `text-9xl`  | 8rem (128px)    | 1           |

### Weight scale

| Token           | Weight | Recommended use        |
| --------------- | ------ | ---------------------- |
| `font-normal`   | 400    | Body, descriptions     |
| `font-medium`   | 500    | UI labels, sm headings |
| `font-semibold` | 600    | Card titles, buttons   |
| `font-bold`     | 700    | h2/h3 (sans), eyebrows |

`font-display` italic ships at 400 only — its calligraphic character comes from the italic style, not weight.

---

## 5. Spacing — Tailwind defaults (unchanged)

4px scale: `space-0` → `space-96`. Document scale only deviates when justified. No custom scale.

---

## 6. Border radius

| Variable        | Computed                           | Use                           |
| --------------- | ---------------------------------- | ----------------------------- |
| `--radius`      | `0.625rem` (10px)                  | Base — cards, popovers        |
| `--radius-sm`   | `calc(var(--radius) - 4px)` = 6px  | Inputs, small buttons, badges |
| `--radius-md`   | `calc(var(--radius) - 2px)` = 8px  | Default buttons               |
| `--radius-lg`   | `var(--radius)` = 10px             | Cards, dialogs                |
| `--radius-xl`   | `calc(var(--radius) + 4px)` = 14px | Hero callouts, large surfaces |
| `--radius-full` | `9999px`                           | Pills, avatars                |

Slightly tighter than the previous `0.65rem` to lean "crafted" without going hard-edged.

---

## 7. Shadow + elevation

Drop shadows are **avoided in dark mode** — petrol-deep backgrounds make them muddy. Dark elevation comes from **lightness step-up** (background → surface → card → popover), not from shadows.

**Light mode** uses standard shadows tinted slightly warm:

| Variable        | Value                                           | Use                      |
| --------------- | ----------------------------------------------- | ------------------------ |
| `--shadow-sm`   | `0 1px 2px 0 oklch(0.18 0.012 195 / 0.06)`      | Cards at rest            |
| `--shadow-md`   | `0 4px 12px -2px oklch(0.18 0.012 195 / 0.10)`  | Cards hover, dropdowns   |
| `--shadow-lg`   | `0 12px 32px -4px oklch(0.18 0.012 195 / 0.14)` | Dialogs, popovers        |
| `--shadow-glow` | `0 0 0 4px oklch(0.79 0.063 158 / 0.18)`        | Focus glow (both themes) |

---

## 8. Motion

| Variable              | Value                            | Use                      |
| --------------------- | -------------------------------- | ------------------------ |
| `--ease-out-quart`    | `cubic-bezier(0.25, 1, 0.5, 1)`  | Default UI transitions   |
| `--ease-in-out-cubic` | `cubic-bezier(0.65, 0, 0.35, 1)` | Page transitions         |
| `--duration-fast`     | `120ms`                          | Hover, focus             |
| `--duration-base`     | `200ms`                          | Card lift, button press  |
| `--duration-slow`     | `360ms`                          | Modal open, drawer slide |

`prefers-reduced-motion: reduce` → snap all transitions to `1ms` (handled in `globals.css`).

---

## 9. Accessibility — verified contrast pairs

Computed against WCAG 2.1 AA (4.5:1 normal, 3:1 large/UI).

### Dark theme

| Pair                                                       | Ratio   | Result |
| ---------------------------------------------------------- | ------- | ------ |
| `#EEE8B2` cream on `#081B1B` ink (body text)               | ~14.8:1 | ✅ AAA  |
| `#EEE8B2` cream on `#203B37` petrol (card text)            | ~11.0:1 | ✅ AAA  |
| `#081B1B` ink on `#5A8F76` sage (primary button label)     | ~5.4:1  | ✅ AA   |
| `#081B1B` ink on `#96CDB0` mint (accent button label)      | ~8.8:1  | ✅ AAA  |
| `#081B1B` ink on `#C18D52` copper (secondary button label) | ~5.8:1  | ✅ AA   |
| `#96CDB0` mint @70% on `#081B1B` (muted text)              | ~6.5:1  | ✅ AA   |

### Light theme

| Pair                                                                       | Ratio   | Result |
| -------------------------------------------------------------------------- | ------- | ------ |
| `#081B1B` ink on `oklch(0.97 0.003 174)` mist (body)                       | ~18.2:1 | ✅ AAA  |
| `#081B1B` ink on `oklch(0.985 0.003 174)` near-white (card text)           | ~19.5:1 | ✅ AAA  |
| `oklch(0.985 0.003 174)` near-white on `#203B37` petrol (primary button)   | ~12.4:1 | ✅ AAA  |
| `#081B1B` ink on `#C18D52` copper (secondary button)                       | ~5.8:1  | ✅ AA   |

### What NOT to do

- **Never** use sage `#5A8F76` as text on cream `#EEE8B2` — 2.1:1 contrast, fails AA. Cream is now a marketing accent only; still applies on any landing surface that uses it.
- **Never** use mint `#96CDB0` as text on petrol `#203B37` for normal-size text — 3.6:1 fails AA at <18pt. Acceptable for large/decorative only.
- **Never** layer copper text on sage bg — clashes hue + insufficient contrast.
- **Never** use `oklch(1 0 0)` pure white for base surfaces — fatiguing on long reads. Always use the near-white `oklch(0.985 0.003 174)` (`--card`) instead.

---

## 10. Quick-reference cheatsheet

```html
<!-- Page scaffold (dark default) -->
<body class="bg-background text-foreground">

<!-- Section header on landing -->
<h1 class="font-display italic text-5xl text-foreground">
  Pagamentos sem ruído.
</h1>

<!-- Card title in dashboard -->
<h3 class="font-serif text-2xl text-card-foreground">
  Recent payment intents
</h3>

<!-- UI label -->
<label class="font-sans text-sm font-medium text-muted-foreground">
  Email
</label>

<!-- Money display -->
<span class="font-mono text-xl text-foreground tabular-nums">
  R$ 1.990,00
</span>

<!-- Primary CTA -->
<button class="bg-primary text-primary-foreground hover:bg-primary/90">
  Connect merchant
</button>

<!-- Premium CTA -->
<button class="bg-secondary text-secondary-foreground hover:bg-secondary/90">
  Upgrade to Pro
</button>

<!-- Status badges -->
<span class="bg-accent text-accent-foreground">captured</span>
<span class="bg-secondary text-secondary-foreground">review</span>
<span class="bg-destructive text-destructive-foreground">failed</span>
```

---

## 11. When to add a new token

Add a token only when **2+ components** need the same value AND the value can't be expressed as a Tailwind utility alias. Avoid one-off semantic vars. Avoid hard-coded HEX in component code — always reach for a semantic var first, fall back to a `--brand-*` raw if marketing/landing truly needs it.

When in doubt, file in `apps/web/docs/design-system/COMPONENTS.md` (separate doc) instead of adding a token.
