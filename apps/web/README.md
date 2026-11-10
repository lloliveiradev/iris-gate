# apps/web

Merchant dashboard and marketing landing page. Next.js 16 (App Router, React 19) talks to `apps/api` through `@iris-gate/sdk-web`. Payment and identity rules stay in `libs/`; this app renders screens, holds the session, and maps form fields onto the HTTP contract.

The API must already be on `http://localhost:8080`. From the repo root:

```bash
make -C apps/web dev
```

Open [http://localhost:3000/en](http://localhost:3000/en). Webpack is the default because Turbopack is heavy on machines with 16 GB of RAM or less. `make dev-turbopack` opts into it. The Makefile sets `NEXT_TELEMETRY_DISABLED=1`. There is no Dockerfile.

## Screens

Locales are `en`, `pt`, and `es`. The default is `en`. `src/middleware.ts` only applies the `next-intl` prefix, so every page lives under `/[locale]`. `<html lang>` uses `pt-BR`, `en`, and `es`.

| Route                                      | What it does                                                    |
| ------------------------------------------ | --------------------------------------------------------------- |
| `/[locale]`                                | Landing: hero, integration, monitoring, reporting, testimonials |
| `/[locale]/login`                          | Accepts an `irk_` API key                                       |
| `/[locale]/dashboard`                      | Overview for the signed-in merchant: recent intents, totals     |
| `/[locale]/dashboard/payment-intents`      | Table (TanStack Table)                                          |
| `/[locale]/dashboard/payment-intents/new`  | Create an intent                                                |
| `/[locale]/dashboard/payment-intents/[id]` | Detail, plus authorize and capture                              |

A missing or unknown locale 404s. Visiting `/` redirects to the default locale.

The dashboard layout is `force-dynamic`. It reads the session and redirects to `/[locale]/login` when `apiKey` or `merchant` is missing. The marketing pages stay public.

## Session and API calls

Login is a server action (`login/actions.ts`). It rejects a blank value and any token that does not start with `irk_`, then calls `GET /v1/me`. On success it stores the key plus `id`, `legal_name`, and `status` in an iron-session cookie and redirects to the dashboard. `401` and `403` surface as an invalid-key error. Logout destroys the cookie.

The cookie is `iris_session` (`IRIS_SESSION_COOKIE`), `httpOnly`, `sameSite=lax`, and `secure` only when `NODE_ENV=production`.

Server code uses two clients in `src/lib/api-client.ts`:

- `publicClient()` — no key
- `authenticatedClient()` — key taken from the session

Browser components never see the key. Lists and mutations go through server actions; TanStack Query in `src/lib/queries/payment-intents.ts` calls those actions and invalidates `['payment-intents']` after authorize or capture.

The create form takes a decimal amount (comma or dot), an ISO-4217 currency (default `BRL`), a `card_token` of at least 8 characters, and an optional description. The action converts the amount to cents with `Math.round(reais * 100)` and sends a fresh `Idempotency-Key` (`crypto.randomUUID()`) on every submit, so two submits create two intents.

## Layout

```text
src/app/[locale]/     routes, server actions next to the screens
src/components/ui/    shadcn (new-york, neutral, lucide)
src/components/       landing, dashboard sidebar, data-table, theme, locale switcher
src/lib/              env, session, API clients, money/date formatting, query hooks
src/i18n/             routing and locale tags
messages/             en.json, pt.json, es.json — same key tree in all three
e2e/                  Playwright
```

Path alias is `@/`. Tailwind 4 is wired from `src/app/globals.css`. Theme light/dark is a class on the document, toggled from `src/components/theme/`.

Visible copy goes through `next-intl`. Add a key to all three files under `messages/`. Money and dates: `pt` formats as `pt-BR`, `en` as `en-US`, `es` as `es-MX`. `formatMoney()` without a locale still formats as `pt-BR`.

## Environment

| Variable                | Default                                                  |
| ----------------------- | -------------------------------------------------------- |
| `IRIS_API_BASE_URL`     | `http://localhost:8080`                                  |
| `IRIS_SESSION_PASSWORD` | a dev-only string of 32+ characters                      |
| `IRIS_SESSION_COOKIE`   | `iris_session`                                           |
| `IRIS_SITE_URL`         | `http://localhost:3000` (canonical and Open Graph links) |

Replace the session password before any deploy. The fallback is fine for localhost.

## Commands

Run from the repo root. `make -C apps/web <target>` matches `yarn` in this workspace.

| Target            | Effect                                                                                                                     |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `dev`             | `next dev --webpack -p 3000`                                                                                               |
| `dev-turbopack`   | `next dev --turbopack -p 3000`                                                                                             |
| `build` / `start` | production build, then `next start -p 3000`                                                                                |
| `lint`            | ESLint flat config                                                                                                         |
| `typecheck`       | `next typegen` and `tsc --noEmit`                                                                                          |
| `test`            | Vitest. The include pattern is `src/**/*.test.ts`; there is no unit test yet, so the run is empty                          |
| `e2e`             | Playwright (Chromium). `webServer` runs `yarn start`, so build first or export `E2E_BASE_URL` to an already running server |
| `clean`           | drops `.next`, Playwright output, and this app's `node_modules`                                                            |

Playwright today checks the English landing headline, the redirect from `/dashboard` to `/login`, and that the API-key field is required.

## Boundaries

- Do not reimplement authorize, capture, or API-key hashing here. Call `@iris-gate/sdk-web` from a server action.
- Do not put the API key in `localStorage`, a client component, or a log line.
- A new screen needs the route under `src/app/[locale]/`, strings in `en` / `pt` / `es`, and a dashboard link only if it should sit behind the session.
- Plans for this app (i18n, tables) live with every other plan in [`docs/plans/`](../../docs/plans/). The design system for this app (tokens, components, accessibility) is in [`docs/design/design-system/`](../../docs/design/design-system/).
