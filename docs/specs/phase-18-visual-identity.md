# Phase 18: Visual identity refresh

**Depends on:** 09. **Status:** **blocked until the owner's design session produces `docs/design/visual-identity.md`.** Closes G10.

## Goal

Replace the current green-heavy theme of the landing page, dashboard, console and hosted checkout with a new visual identity defined by the project owner.

## What is known now

- The owner finds the current palette too green: green appears in too many places, including green-tinted white backgrounds (`--background: oklch(0.97 0.003 174)` in `apps/web/src/app/globals.css`).
- Backgrounds and large surfaces should be neutral. Color is for brand, primary actions and status.
- The theme is defined as OKLCH tokens in `globals.css` (Tailwind 4 + shadcn). The change should happen mostly at the token level.

## Before starting

Do not start this phase until `docs/design/visual-identity.md` exists and its status says approved. If it is missing, open an owner card asking for the design session and work on another phase.

## In scope (once unblocked)

1. Tokens: palette, light and dark themes, semantic tokens (background, surface, border, text, primary, accent, success, warning, danger, review) as the visual-identity doc defines.
2. Apply to the landing page, dashboard, console and hosted checkout. The hosted checkout keeps the merchant's brand color for its primary button, on top of the neutral base.
3. Status colors for payment states, consistent across lists, badges and timeline.

## Guidelines

- WCAG 2.2 AA contrast for text and controls in both themes; check it with an automated test (for example axe in Playwright).
- No hard-coded colors in components; only tokens.
- Screenshots before and after in the PR for each main page, in both themes.

## Acceptance criteria

1. No component uses a literal color.
2. Axe reports no contrast violations on the main pages in both themes.
3. The owner approves the screenshots (owner card).
