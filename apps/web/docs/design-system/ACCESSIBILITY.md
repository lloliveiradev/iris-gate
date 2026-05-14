# IrisGate — Accessibility

> Target: WCAG 2.1 AA across the product. AAA where the contrast tables in `TOKENS.md` show we already clear it (body text in both themes). Anything below AA is a bug, not a tradeoff.

---

## 1. Contrast

Always verify against tokens, not perceived screen colors. Reference table lives in `TOKENS.md §9`.

Current pairs that ship today:

**Dark theme**
- `--brand-cream` body on `--brand-ink` bg → ~14.8:1 ✅ AAA
- `--brand-cream` body on `--brand-petrol` card → ~11.0:1 ✅ AAA
- `--muted-foreground` (solid mint 0.82L) on `--background` → ~7.1:1 ✅ AAA
- `--brand-ink` on `--brand-sage` (primary button label) → ~5.4:1 ✅ AA
- `--brand-ink` on `--brand-copper` (secondary button label) → ~5.8:1 ✅ AA

**Light theme**
- `--brand-ink` on `oklch(0.97 0.003 174)` mist (body) → ~18.2:1 ✅ AAA
- `--brand-ink` on `oklch(0.985 0.003 174)` near-white (card text) → ~19.5:1 ✅ AAA
- `oklch(0.985 0.003 174)` near-white on `--brand-petrol` (primary button) → ~12.4:1 ✅ AAA
- `--muted-foreground` (solid petrol 0.36L) on mist → ~6.3:1 ✅ AA

**Banned pairs:**
- Sage `#5A8F76` text on cream `#EEE8B2` — 2.1:1, fails AA. (Cream is marketing-accent only now, but the rule still applies on landing surfaces.)
- Mint `#96CDB0` text on petrol `#203B37` for normal-size copy — 3.6:1, fails AA. Large/decorative only.
- Pure white `oklch(1 0 0)` as a base surface — fatiguing on long reads.

If you need to introduce a new color combination, compute the ratio (any OKLCH-aware tool, or convert the OKLCH endpoints to hex and use WebAIM Contrast Checker). Document the result in `TOKENS.md §9`.

---

## 2. Focus visibility

**The rule:** every interactive element shows a visible focus indicator on keyboard navigation. Mouse users see no ring (`:focus-visible`, not `:focus`).

The shadcn-derived pattern is:

```css
focus-visible:outline-none
focus-visible:border-ring
focus-visible:ring-ring/50
focus-visible:ring-[3px]
```

`--ring` is **mint** in dark, **sage** in light — the most saturated tokens we have. This is intentional: focus is the one place we sacrifice subtlety. Do not override per-component.

Globally, `globals.css` also sets `outline-ring/50` on every element as a safety net so a missed focus style doesn't ship invisible.

Custom components must call `focus-visible:` not `focus:` so the ring only appears for keyboard / programmatic focus, not mouse clicks.

---

## 3. Color is never the only channel

Status, error, and required-field signals always carry **a label** alongside the color.

- Status badges: the variant color reinforces a textual status (`"captured"`, `"failed"`). The text is the source of truth.
- Form errors: a `text-destructive` message under the input. **Do not** turn the input border red without also rendering the message.
- Required fields: an explicit asterisk `*` after the label (or the word "Required"), plus `required` HTML attribute. Not a color cue.

Colorblind users (~8% of men), low-vision users, and anyone in bright sunlight relies on this.

---

## 4. Keyboard navigation

Every interaction must be reachable without a mouse.

- **Tab order** follows source order. If you need to deviate, the deviation is usually a layout bug, not a tabindex bug. Avoid `tabindex` > 0.
- **Skip-link** — `<a href="#main" className="sr-only focus:not-sr-only ...">Skip to content</a>` should be the first focusable element on every page. Not yet wired site-wide — flag this as a pending item when you next touch `RootLayout`.
- **Dialog/Popover** — focus is trapped on open, restored to the trigger on close, `Escape` always closes. Radix-based shadcn primitives handle this for free; do not bypass them by building from scratch.
- **Tables** — header cells `<TableHead>` use `<th scope="col">` (the shadcn primitive does this). Cells with row-actions should expose a button, not a `div onClick`.

---

## 5. Screen reader text

- Icon-only buttons require `aria-label` on the button + `aria-hidden` on the icon. Example: `ThemeToggle` ships this pattern — copy it.
- Decorative icons inside text labels: `aria-hidden` on the icon, no extra label.
- Visually hidden labels: use `sr-only` (Tailwind). Do not use `display: none` (removes from a11y tree) or `visibility: hidden`.
- Dynamic content updates (toast, form submission result): wrap in `aria-live="polite"` (or `assertive` for errors). Form-level errors should use `role="alert"`.

The `sr-only` utility is **not currently in use** anywhere in the app — when you need it, that's a sign you're shipping a new icon-only control or a skip-link.

---

## 6. Reduced motion

Handled in CSS, no JS required:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 1ms !important;
    transition-duration: 1ms !important;
    animation-iteration-count: 1 !important;
  }
}
```

Already shipped in `globals.css`. Honors the OS-level setting on every transition and animation.

If you build a motion via `framer-motion` or `requestAnimationFrame`, also check `matchMedia('(prefers-reduced-motion: reduce)').matches` and short-circuit. CSS-only animations are preferred for this reason.

No parallax, no auto-playing video, no auto-advancing carousels. If a feature needs motion to communicate meaning, it needs a no-motion fallback that still communicates.

---

## 7. Forms

(Also see `PATTERNS.md §4`.) The accessibility checklist for every form field:

- `<label htmlFor="...">` paired with the input — always visible, `sr-only` only as last resort.
- `aria-invalid={hasError}` when the field is in error.
- `aria-describedby="<id>-err"` when there's an error message; the message uses that `id`.
- Server errors at form-level live in `role="alert"`.
- `required` on the HTML attribute, asterisk in the label.
- `inputMode` set appropriately (`numeric`, `decimal`, `email`, `tel`) so mobile keyboards make sense.

---

## 8. Theme toggle and high-contrast preferences

The `ThemeToggle` persists to `localStorage.iris-theme`. We do **not** auto-detect `prefers-color-scheme` on first paint — dark is the explicit default for everyone, and users opt into light.

If we later add an "auto" mode, the pattern is:

1. `ThemeScript` reads `localStorage.iris-theme`. If `"auto"` or empty, read `matchMedia('(prefers-color-scheme: light)').matches`.
2. Re-evaluate on `matchMedia` `change` event.
3. Store the user's manual override the moment they touch the toggle — don't keep them in "auto" silently.

`prefers-contrast: more` is not yet supported. When we add it, the most likely change is bumping `--muted-foreground` to a darker shade and tightening `--border` opacity. Document in `TOKENS.md` when implemented.

---

## 9. Testing checklist before a feature ships

Run through this for any new page or major component:

- [ ] Tab through every interactive element — every one shows the focus ring.
- [ ] Tab order is logical (top-to-bottom, left-to-right by visual position).
- [ ] All icon-only buttons have an `aria-label`.
- [ ] All form fields have a visible `<label>` (or a documented `sr-only` exception).
- [ ] Error states render both a message AND a color cue, not color alone.
- [ ] Status indicators have text labels alongside colors.
- [ ] Dialog/Popover: trap focus, close on Escape, restore focus on close.
- [ ] Toggle the OS reduced-motion preference — confirm transitions snap.
- [ ] Toggle to light theme — confirm contrast still passes (especially `--muted-foreground` on bg).
- [ ] Zoom to 200% — layout doesn't break, no horizontal scroll.
- [ ] Run axe DevTools on the page — zero violations.

---

## 10. When in doubt

Defer to: WAI-ARIA Authoring Practices Guide → MDN ARIA pages → shadcn's underlying Radix primitive. If shadcn ships it and Radix made it, the a11y has been done; the failure mode is wrapping it in a way that breaks the contract (e.g., killing focus trap by re-mounting the trigger).
