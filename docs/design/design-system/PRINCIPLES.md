# IrisGate — Design Principles

> The "why" behind the tokens. Read this once before contributing UI; consult `TOKENS.md` for "what" and `COMPONENTS.md` for "how".

---

## 1. Personality — artisanal fintech

Most fintech UI looks like the same Stripe-blue spreadsheet wearing different logos. IrisGate intentionally rejects corporate-blue defaults:

- **Petrol + sage** carry the trust backbone (instead of cold blue).
- **Copper** lifts CTAs and premium moments — assertive, warm, never timid.
- **Mint** is the soft "yes" — captured, approved, focus glow.
- **Cream** is the editorial inversion — dark-mode body text, marketing accents, never base UI in light mode.
- **Ink** is the deepest petrol — page bg in dark mode.

The system should feel **considered**, like good print typography on a good monitor. Curved corners (10px base), warm shadows, italicized display serif, generous line-height. Anti-skeuomorphic, anti-glassy, anti-neon.

---

## 2. Dark is the default

`<html class="dark">` ships in `layout.tsx`. Light mode is opt-in via the in-header `ThemeToggle` (persisted to `localStorage.iris-theme`). Pre-paint inline script in `ThemeScript` prevents FOUC.

Rationale:
- Long-session ops tools (payments, fraud) live on the dashboard — dark is easier on eyes for extended reads.
- The brand palette is dark-native (ink + petrol carry the identity).
- Light is provided for daylight / accessibility preference, not as the primary canvas.

When designing a new surface: **design dark first**, then verify it inverts cleanly via tokens. If a surface looks great dark but breaks light, the surface is using brand primitives instead of semantic tokens (see §4).

---

## 3. Elevation is lightness, not shadow

Dark mode uses a **3-tier lightness step-up** for elevation:

```
L1 background  → --brand-ink           (deepest)
L2 surface     → --brand-petrol-mid    (panels, sidebars, table headers)
L3 card        → --brand-petrol        (lifted content)
```

Drop shadows go muddy on petrol-deep backgrounds. Do not add them in dark mode. Use background step-up.

Light mode does use shadows — but they're subtle (`--shadow-sm/md/lg`) with petrol-ink-tinted alpha, not pure-black. Light surfaces also use a 3-tier system, just shifted to near-white:

```
L1 background  → oklch(0.97  0.003 174) — mist
L2 surface     → oklch(0.955 0.004 174) — one step down
L3 card        → oklch(0.985 0.003 174) — elevated above bg
```

**Never `oklch(1 0 0)` (pure white) for base surfaces.** Fatiguing on long reads.

---

## 4. Semantic tokens before brand primitives

Two layers in `globals.css`:

- **Brand primitives** — `--brand-ink`, `--brand-petrol`, `--brand-sage`, `--brand-mint`, `--brand-copper`, `--brand-cream`, `--brand-cream-soft`, `--brand-petrol-mid`. The raw palette.
- **Semantic tokens** — `--background`, `--card`, `--primary`, `--accent`, `--muted-foreground`, etc. Components consume these.

**The rule:** product UI uses semantic tokens. Marketing/landing may reach for brand primitives when the moment is editorial (e.g., copper italic accent in the Hero h1). If a product component touches `--brand-*` directly, that's a code-smell — the theme can't invert.

Tailwind aliases for both layers are wired in the `@theme inline` block (`bg-background`, `text-brand-copper`, etc.) — so the discipline is "which alias did you reach for?", not "did you hard-code a hex?".

---

## 5. Typography — 4 families, 1 hierarchy

| Family | Variable | When |
|---|---|---|
| Geist Sans | `--font-sans` | Body, UI labels, buttons, inputs |
| Fraunces | `--font-serif` | h2 / h3 / card titles — warm editorial weight |
| Instrument Serif (italic) | `--font-display` | h1 hero, marketing display |
| Geist Mono | `--font-mono` | Money, IDs, code, timestamps — always with `tabular-nums` |

The display serif **is the brand's voice**. Use it sparingly — one h1 per page in marketing surfaces, none in dashboards. Fraunces does the heavier lifting in product UI.

Default to `font-sans` unless there's a reason to escalate. Reasons: editorial heading, monetary value, code/ID. Otherwise sans.

---

## 6. Motion — fast, eased, respectful

Three durations, two eases:

- `--duration-fast` (120ms) — hover/focus.
- `--duration-base` (200ms) — card lift, button press, popover open.
- `--duration-slow` (360ms) — modals, drawers.
- `--ease-out-quart` for entrances; `--ease-in-out-cubic` for transitions that both enter and exit.

Reduced-motion is non-negotiable: `@media (prefers-reduced-motion: reduce)` in `globals.css` already snaps every transition to 1ms. **Do not** add new motion via JS-driven timeouts that bypass this — if you must animate in JS, also read `matchMedia('(prefers-reduced-motion: reduce)')`.

No bouncing, no parallax, no auto-scrolling carousels. Calm motion or none.

---

## 7. Focus is loud, on purpose

`--ring` is mint in dark, sage in light — the most saturated colors in each theme. Focus is the one place we sacrifice subtlety. Default shadcn pattern is wired:

```css
focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]
```

Do **not** override per-component. If a focus ring clashes with a custom surface, the surface needs to change, not the ring.

---

## 8. Brand surfaces vs product surfaces

| Surface type | Lives in | Tokens to use | Notes |
|---|---|---|---|
| Marketing / landing | `components/landing/`, `components/site/`, `app/page.tsx` | Semantic + may reach for `--brand-*` for editorial accents | Hero italic copper highlight is fine; cream marketing background is fine |
| Product / dashboard | `app/dashboard/**`, `components/dashboard/` | Semantic only — `bg-card`, `text-foreground`, `text-muted-foreground` | Must invert cleanly across both themes. No `--brand-cream` as background. |
| Auth (login) | `app/login/` | Semantic only | Treat as product UI |

When in doubt, the test is: *does this surface need to render in both themes?* If yes, semantic tokens only.

---

## 9. Decisions log

- **2026-05-11** — adopted petrol/sage/copper/cream palette. Display serif: Instrument Serif italic. Body: Geist Sans.
- **2026-05-11** — `--muted-foreground` made solid (no alpha) in both themes after AA failure on alpha-blended mint.
- **2026-05-11** — ThemeToggle + pre-paint `ThemeScript` to prevent FOUC.
- **2026-05-13** — light-mode base surfaces moved from cream/white to petrol-tinted near-white off-grays. Cream is now marketing-only.
