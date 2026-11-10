# Plan: web tables with TanStack Table and TanStack Query

> **Status: rollout phase 1 implemented in phase 5 (2026-05); rollout phases 2 and 3 pending.** Moved from `apps/web/docs/plans/tables.md` on 2026-11-10. The `DataTable` shell lives in `apps/web/src/components/data-table/` and the payment-intents table in `apps/web/src/app/[locale]/dashboard/payment-intents/`. Routes now sit under `[locale]/`. Server-side pagination and filters (section 3, rollout phase 2) are still open; pick them up when a delivery phase needs bigger lists (phase 11 adds status filters, including `requires_review`). The row actions in section 4.5 change in phase 11: Authorize is removed, and Capture, Cancel and Refund follow the new lifecycle.

> Goal: a reusable, feature-complete table system for every authenticated list view (payment intents first, then merchants, webhooks, API keys, audit log…). Single composable `DataTable` shell, headless state via TanStack Table, server state via TanStack Query, UI via shadcn primitives (with patterns lifted from Origin UI). Each call site picks the features it needs from a known menu.

---

## 0 · TL;DR — what we'll build

```
<DataTable
  data={intents}
  columns={paymentIntentColumns}
  pagination={{ pageSize: 25 }}
  toolbar={{ search: true, filters: ['status'], viewOptions: true }}
  rowActions={paymentIntentRowActions}
  isLoading={isLoading}
/>
```

Composed of:

- `DataTable` — orchestrator (table + toolbar + pagination)
- `DataTableToolbar` — search input + faceted filters + view options
- `DataTablePagination` — page nav + size selector + selection count
- `DataTableColumnHeader` — sortable column header with sort icon
- `DataTableViewOptions` — column visibility toggle (dropdown)
- `DataTableRowActions` — ellipsis dropdown for per-row actions
- `useListQuery` / `useEntityMutation` — TanStack Query hooks per entity

Underneath:

- `@tanstack/react-table` (headless table primitives)
- `@tanstack/react-query@5.62` (already installed)
- shadcn `Table`, `Input`, `Select`, `DropdownMenu` (Radix — already installed)
- Server Actions as the data plumbing (keeps API key server-only)

---

## 1 · Current state (audit findings)

- **TanStack Table** is NOT installed. **TanStack Query 5.62.10 is** installed and `QueryProvider` is already mounted in the dashboard layout.
- **`@radix-ui/react-dropdown-menu` 2.1.4** is installed; shadcn `dropdown-menu.tsx` is NOT yet generated.
- **No `useQuery` / `useMutation` hooks exist** anywhere — clean slate.
- **`api-client.ts` is `server-only`** (`import 'server-only'`). Client components can't import the SDK directly because the API key lives in an encrypted cookie on the server. So data hooks must call **Server Actions**, not the SDK directly.
- **API list endpoint capabilities** (`GET /v1/payment-intents`):
  - Today: `limit` only (1–200, default 50).
  - Not supported yet: `cursor` / `offset` / `page` / `status` filter / `search` / `sort`.
  - Returns the full array, no envelope, no `total_count`, no `next_cursor`.
- **OpenAPI contract** lives at `libs/contracts/http/payments.openapi.yaml`. Any pagination params must be added there first — types are generated from it.
- **SDK** (`@iris-gate/sdk-web` → `libs/sdk-web/src/client.ts`): `listPaymentIntents(): Promise<PaymentIntent[]>` takes no params. Needs to accept a request shape.

---

## 2 · Stack decisions

| Concern | Choice | Why |
|---|---|---|
| Headless table state | `@tanstack/react-table` v8 | De-facto standard, framework-agnostic, integrates cleanly with shadcn Table primitive |
| Server state | `@tanstack/react-query` v5 | Already installed; offers caching, request dedup, optimistic updates, invalidation |
| UI shell | shadcn `Table` + `DropdownMenu` + `Input` + `Select` | Already using shadcn; consistent with the rest of the system |
| Per-row actions | Radix `DropdownMenu` ellipsis trigger | Standard pattern (Stripe, Linear, Vercel) |
| Data plumbing | **Server Actions**, not Route Handlers | Keep API key server-only; SAs are typesafe, composable, easy to mock |
| Pagination model | **Cursor-based** (V1 supports basic offset; V2 cursor) | Monotonic intent IDs, scales well, no `COUNT(*)` cost |
| Filtering | Server-side on the API; client-side fallback for V1 | Filtering local-only is OK while limit=50; promote to server when lists outgrow that |
| Sorting | Server-side; default `created_at DESC` | Same reasoning as filtering |
| Selection | TanStack row selection state, exposed via `getIsAllPageRowsSelected` / `getSelectedRowModel` | Standard pattern |

---

## 3 · Server-side enhancements needed

The API has to grow up before we can wire a real paginated/searchable/filterable table. Order of work:

### 3.1 · OpenAPI contract change

`libs/contracts/http/payments.openapi.yaml` `paths./v1/payment-intents.get`:

```yaml
parameters:
  - in: query
    name: limit
    schema: { type: integer, minimum: 1, maximum: 100, default: 25 }
  - in: query
    name: cursor          # opaque base64 of {created_at, id} — backend decodes
    schema: { type: string }
  - in: query
    name: status          # CSV list of statuses to include
    schema: { type: string, example: "pending,authorized" }
  - in: query
    name: q               # free-text search; matches intent_id prefix
    schema: { type: string }
  - in: query
    name: created_after
    schema: { type: string, format: date-time }
  - in: query
    name: created_before
    schema: { type: string, format: date-time }
responses:
  '200':
    schema:
      type: object         # NEW: envelope, not raw array
      properties:
        data:        { type: array, items: { $ref: '#/components/schemas/PaymentIntent' } }
        next_cursor: { type: string, nullable: true }
        has_more:    { type: boolean }
```

This is a breaking change for the SDK. Worth it — every other list endpoint will need the same envelope, so do it once.

### 3.2 · Go handler change

`libs/payments/infra/httpapi/handlers.go` `handleListPaymentIntents`:

- Parse new query params.
- Pass them to `repo.ListByMerchant(ctx, merchantID, filter)` where `filter` is a struct.
- Compute `next_cursor` from the last row's `(created_at, id)`.
- Wrap response in `{ data, next_cursor, has_more }`.

`PaymentIntentRepo.ListByMerchant` signature changes from `(ctx, merchantID, limit) → []PaymentIntent` to `(ctx, merchantID, ListFilter) → ListResult`.

### 3.3 · SQL changes

- Existing index on `(merchant_id, created_at DESC, id DESC)` — confirm it exists; add if not. Keyset pagination needs this exact ordering.
- `WHERE merchant_id = $1 AND (created_at, id) < ($cursor_created_at, $cursor_id) AND status = ANY($statuses) AND ($q IS NULL OR id LIKE $q || '%') ORDER BY created_at DESC, id DESC LIMIT $limit + 1`.
- `+1` row drives `has_more` without an extra query.

### 3.4 · SDK update

`libs/sdk-web/src/client.ts`:

```ts
async listPaymentIntents(params: ListPaymentIntentsRequest = {}): Promise<ListPaymentIntentsResponse> {
  const qs = buildQuery(params); // URLSearchParams from {limit, cursor, status, q, ...}
  return this.request('GET', `/v1/payment-intents${qs ? `?${qs}` : ''}`);
}
```

Types come from `@iris-gate/contracts` (regenerated from the updated OpenAPI).

---

## 4 · Web wiring

### 4.1 · Server Actions per entity

`apps/web/src/app/dashboard/payment-intents/queries.ts` (server module):

```ts
'use server';

import { authenticatedClient } from '@/lib/api-client';
import type { ListPaymentIntentsRequest } from '@iris-gate/sdk-web';

export async function listPaymentIntents(params: ListPaymentIntentsRequest) {
  const client = await authenticatedClient();
  return client.listPaymentIntents(params);
}
```

Same shape for mutations:

```ts
export async function authorizePaymentIntent(id: string) { /* ... */ }
export async function capturePaymentIntent(id: string) { /* ... */ }
```

### 4.2 · TanStack Query hooks

`apps/web/src/lib/queries/payment-intents.ts`:

```ts
import { keepPreviousData, useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listPaymentIntents, authorizePaymentIntent, capturePaymentIntent } from '@/app/dashboard/payment-intents/queries';

export const paymentIntentKeys = {
  all: ['payment-intents'] as const,
  list: (params: ListPaymentIntentsRequest) => ['payment-intents', 'list', params] as const,
  detail: (id: string) => ['payment-intents', 'detail', id] as const,
};

export function useListPaymentIntents(params: ListPaymentIntentsRequest) {
  return useQuery({
    queryKey: paymentIntentKeys.list(params),
    queryFn: () => listPaymentIntents(params),
    placeholderData: keepPreviousData, // smooth pagination — old page stays while new loads
  });
}

export function useAuthorizePaymentIntent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: authorizePaymentIntent,
    onSuccess: () => qc.invalidateQueries({ queryKey: paymentIntentKeys.all }),
  });
}
```

### 4.3 · DataTable shell (client component)

`apps/web/src/components/data-table/data-table.tsx` exports the orchestrator. Internally uses `useReactTable({ data, columns, state, onPaginationChange, manualPagination: true, pageCount: -1, getCoreRowModel, ... })`.

Manual pagination + `manualFiltering` + `manualSorting` so TanStack Table is **purely a state holder** — the server is the source of truth. The `state` (pagination, sorting, columnFilters, columnVisibility) lives in URL params (via `nuqs` — recommended, see §7 open questions) so deep links work.

### 4.4 · Per-entity column factories

`apps/web/src/app/dashboard/payment-intents/columns.tsx` exports `paymentIntentColumns: ColumnDef<PaymentIntent>[]`:

```tsx
export const paymentIntentColumns: ColumnDef<PaymentIntent>[] = [
  {
    id: 'select',
    header: ({ table }) => <Checkbox checked={table.getIsAllPageRowsSelected()} onCheckedChange={...} />,
    cell: ({ row }) => <Checkbox checked={row.getIsSelected()} onCheckedChange={...} />,
    enableSorting: false,
    size: 32,
  },
  {
    accessorKey: 'id',
    header: ({ column }) => <DataTableColumnHeader column={column} title="ID" />,
    cell: ({ row }) => <span className="font-mono text-xs text-muted-foreground">{row.original.id}</span>,
  },
  {
    accessorKey: 'amount',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('fields.amount')} align="right" />,
    cell: ({ row }) => (
      <span className="font-mono tabular-nums font-medium">
        {formatMoney(row.original.amount.amount_cents, row.original.amount.currency)}
      </span>
    ),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => <Badge variant={statusVariant(row.original.status)} dot>{row.original.status}</Badge>,
    filterFn: 'equals', // for the faceted status filter
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('fields.created_at')} />,
    cell: ({ row }) => <span className="text-muted-foreground">{formatDateTime(row.original.created_at)}</span>,
  },
  {
    id: 'actions',
    cell: ({ row }) => <PaymentIntentRowActions intent={row.original} />,
    enableSorting: false,
    size: 48,
  },
];
```

### 4.5 · Row actions (ellipsis dropdown)

`PaymentIntentRowActions` opens a `DropdownMenu` with:
- "View details" (link to the detail page)
- "Authorize" (mutation, disabled if not pending)
- "Capture" (mutation, disabled if not authorized)
- separator
- "Copy ID" (copy intent.id to clipboard)
- separator
- "Cancel" (destructive, requires Dialog confirmation)

Labels come from the `dashboard.payment_intents.actions.*` messages. Trigger: `<Button variant="ghost" size="icon"><MoreHorizontal /></Button>` with a translated `aria-label` ("Actions").

Pattern is well-covered by Origin UI's table-row-actions samples (e.g., their "table-19" / "table-20" examples — see Origin UI docs for the exact one to crib from).

---

## 5 · Building blocks — files to create

```
apps/web/src/
├── components/
│   ├── ui/
│   │   ├── dropdown-menu.tsx       NEW — npx shadcn add dropdown-menu
│   │   ├── checkbox.tsx            NEW — npx shadcn add checkbox
│   │   ├── select.tsx              NEW — npx shadcn add select
│   │   └── skeleton.tsx            NEW — npx shadcn add skeleton
│   └── data-table/
│       ├── data-table.tsx          orchestrator (useReactTable, layout)
│       ├── toolbar.tsx             search + filters + view options
│       ├── pagination.tsx          page nav + size selector + selection count
│       ├── column-header.tsx       sortable header w/ ChevronUp/Down icons
│       ├── view-options.tsx        column visibility toggle
│       ├── row-actions.tsx         ellipsis dropdown wrapper
│       ├── faceted-filter.tsx      multi-select chip filter (status, etc.)
│       └── empty-state.tsx         no-results / no-data variants
├── lib/
│   ├── queries/
│   │   └── payment-intents.ts      tanstack-query hooks per entity
│   └── url-state.ts                nuqs helpers (page, q, status filter)
└── app/dashboard/payment-intents/
    ├── queries.ts                  Server Actions (use server)
    ├── columns.tsx                 column definitions
    └── page.tsx                    becomes a thin wrapper around DataTable
```

---

## 6 · Feature matrix per table

| Feature | Payment intents (V1) | Merchants | API keys | Audit log |
|---|---|---|---|---|
| Search | id-prefix | name | label | actor + action |
| Faceted filter | status, date range | status | scope (sandbox/live) | severity, action type |
| Sortable columns | amount, created_at | created_at | created_at, last_used_at | created_at |
| Row selection | yes (bulk capture/cancel — V2) | no | yes (revoke bulk) | no |
| Row actions | View / Authorize / Capture / Cancel / Copy ID | Edit / Suspend | Revoke / Roll | View detail |
| Pagination | cursor, 25/page | cursor, 50/page | cursor, 25/page | cursor, 100/page |
| Density toggle | optional | optional | optional | yes (compact default) |
| Export CSV | V3 | V3 | no | V2 |

You'll pick the boxes per surface as we build each one. The DataTable shell will support the union of features and each call site enables what it needs.

---

## 7 · Open questions

1. **URL state library** — propose `nuqs` (lightweight, types, App Router-native). Alternative: roll our own `useSearchParams` wrapper. nuqs adds one dep but pays for itself by the second table.
2. **Realtime updates** — should the list auto-refetch when a mutation in a Server Component flushes? React Query's `invalidateQueries` handles this when the mutation is run client-side. Server-action-only flows need an explicit `router.refresh()` or a query invalidation key passed back.
3. **Optimistic updates** — for status transitions (Authorize / Capture), do we go optimistic? Decision: **yes** for status changes (UI flips immediately, rolls back on error); **no** for create (need server-generated ID).
4. **Skeleton density** — show 10 skeleton rows on first load (full table height) or just 3 (compact)? Default: match `pageSize`, so the table doesn't jump.
5. **CSV export** — server-side (API endpoint returning CSV stream) or client-side from current page? Recommend server-side once it's wanted, with a streaming download. Out of V1.
6. **Bulk actions UX** — once row selection exists, where do bulk buttons live? Top toolbar (a "with N selected" replacement bar), or as floating action bar at the bottom? Stripe uses replacement-bar; Linear uses floating. Tally up before V2.

---

## 8 · Phased rollout

**Phase 1 — Foundation** (single PR, no API changes)
- Install `@tanstack/react-table`.
- Generate shadcn `dropdown-menu`, `checkbox`, `select`, `skeleton`.
- Build `DataTable` shell with **client-side** pagination/sort/filter against the existing `limit=50` payload.
- Migrate the existing `/dashboard/payment-intents` table to the new shell (visual parity, no new features visible to user yet).
- Row actions dropdown replaces the "Details" button. Includes Authorize/Capture as mutations using existing Server Actions.

**Phase 2 — Server pagination + search** (API + SDK + web)
- Ship the OpenAPI / handler / SDK envelope change.
- Switch `useListPaymentIntents` to manual mode (`manualPagination: true`).
- Add search input + status faceted filter + date range.
- URL state via nuqs so deep links work.

**Phase 3 — Polish + extend**
- Add Skeleton rows on first load.
- Column visibility toggle.
- Optimistic status updates.
- Repeat the same shell for the next table that lands (Merchants? API keys?).

Each phase ships independently. Phase 1 is the meaty one because it establishes the shell; Phases 2/3 are config + features on top.
