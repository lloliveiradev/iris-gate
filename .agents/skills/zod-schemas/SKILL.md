---
name: zod-schemas
description: >
  Generate Zod 4 validation schemas for the Web app. Schemas are runtime
  validators that **mirror the DTO types owned by the `django-to-typescript` skill** — they
  do not redefine or re-export the DTO. Use this skill whenever the user asks to
  create Zod schemas, validation schemas, form validation, or runtime type
  checking. Also trigger when the user mentions "zod schema", "validate DTO",
  "form schema", "nuqs schema", or "runtime validation". Pair with `django-to-typescript`:
  `django-to-typescript` owns all type contracts (Create/Update DTOs, Response/entity
  interfaces); `zod-schemas` owns runtime validation only.
---

# Zod 4 Schema Generation Skill

Generate Zod 4 schemas as **runtime validators that mirror DTO types**. Type
contracts (DTOs and Response/entity interfaces) live in the `types/` layer
(owned by the `django-to-typescript` skill). The schema layer never redefines or owns
those types — it provides the runtime validator.

---

## Source of Truth

| Layer                                       | Owner             | Purpose                                    |
| ------------------------------------------- | ----------------- | ------------------------------------------ |
| All type contracts (Create/Update/Response) | `django-to-typescript` skill | `types/<entity>-types.ts` — single owner   |
| Runtime validators                          | `zod-schemas`     | `schemas/<entity>-schema.ts` — mirrors DTO |
| Backend authoritative shape                 | Django serializer | Read first; flows into `django-to-typescript`         |

**Always look at the type when writing a schema.** Open `types/<entity>-types.ts`,
read the `Create[Entity]DTO` / `Update[Entity]DTO`, then write a Zod schema that
mirrors it field-by-field. When the DTO type changes, update the schema in the
same commit.

> **Do not use `satisfies z.ZodType<DTO>`.** It strips Zod's type chain, breaks
> `.partial()` / `z.input` / `z.output`, and adds friction. Skip it.
>
> **Do not re-export DTOs from the schema file.** Consumers continue to import
> `Create[Entity]DTO` from the `types/` barrel. The schema file exports schemas
> only.
>
> **`z.infer<typeof Schema>` is allowed inline** when a specific call-site needs
> the parsed output type (e.g., the return type of `safeParse`, or with
> transforms). Do not export an inferred type that duplicates the DTO.
>
> **Do not generate Response schemas.** Backend is typed; runtime response
> parsing adds overhead with no benefit. Response shapes stay in `django-to-typescript`.

---

## File Naming & Structure

Files live under `domains/<context>/schemas/`, following the `web-domain-structure`.

| What            | Convention                        | Example                                            |
| --------------- | --------------------------------- | -------------------------------------------------- |
| Schema file     | `[entity]-schema.ts` (kebab-case) | `domains/articles/schemas/article-schema.ts`       |
| Schema variable | `[Action][Entity]Schema` (Pascal) | `CreateArticleSchema`                              |
| DTO type        | imported from `../types`          | `import type { CreateArticleDTO } from "../types"` |

Always open generated code with the file path as a comment header:

```ts
// domains/<context>/schemas/article-schema.ts
```

One schema file per entity. All re-exported from a barrel `index.ts`:

```ts
// domains/<context>/schemas/index.ts
export { CreateArticleSchema, UpdateArticleSchema } from "./article-schema";
export { CreateOrderSchema, UpdateOrderSchema } from "./order-schema";
```

> Schemas are runtime values — use plain `export`. Do not export DTO types from
> here; they live in `types/`.

---

## Schema Layers

For every entity, generate exactly **two** schemas, both mirroring the DTO type
from `types/<entity>-types.ts`:

| Schema                 | Derivation                                            | Purpose          |
| ---------------------- | ----------------------------------------------------- | ---------------- |
| `Create[Entity]Schema` | hand-written `z.object({ … })` mirroring `CreateXDTO` | POST validation  |
| `Update[Entity]Schema` | derived from Create, mirroring `UpdateXDTO`           | PATCH validation |

```ts
// schemas/article-schema.ts
import { z } from "zod";

export const CreateArticleSchema = z.object({
  /* mirror CreateArticleDTO */
});
export const UpdateArticleSchema = CreateArticleSchema.partial();
```

If `UpdateXDTO` extends `Partial<CreateXDTO>` with extra required fields (e.g.,
`& { id: string }` so consumers can read it back when building URLs), use
`.partial().extend({ … })`:

```ts
export const UpdateArticleSchema = CreateArticleSchema.partial().extend({
  id: z.uuid(),
});
```

When a specific call-site needs the schema's parsed output type (e.g., to type
the result of `safeParse`), use `z.infer<typeof Schema>` **inline at that
call-site** — do not export it from the schema file.

```ts
// in a service or hook (NOT in the schema file)
const result = CreateArticleSchema.safeParse(input);
if (result.success) {
  const validated: z.infer<typeof CreateArticleSchema> = result.data;
}
```

When the schema has transforms / `z.stringbool()` / `z.coerce.*` (input ≠ output),
use `z.input` / `z.output` inline at the consuming code (typically a form's
`useForm` defaults vs. submit handler).

---

## TypeScript → Zod 4 Mapping Table

Uses **Zod 4 native APIs**: top-level format helpers, `z.literal([…])` for
multi-literal, `z.iso.*` for ISO formats.

| Field origin                          | Zod 4 schema                            | Notes                                                   |
| ------------------------------------- | --------------------------------------- | ------------------------------------------------------- |
| `string`                              | `z.string()`                            |                                                         |
| UUID (`id`, `*_id` PK/FK)             | `z.uuid()`                              |                                                         |
| email                                 | `z.email()`                             |                                                         |
| URL (`FileField`, `ImageField`)       | `z.url()` or `z.string()`               | Use `z.string()` when backend may return relative paths |
| HTTP/HTTPS-only URL                   | `z.httpUrl()`                           |                                                         |
| ISO datetime (`*_at`)                 | `z.iso.datetime({ offset: true })`      | Response only — Create/Update rarely include `*_at`     |
| ISO date / time                       | `z.iso.date()` / `z.iso.time()`         |                                                         |
| IPv4 / IPv6                           | `z.ipv4()` / `z.ipv6()`                 |                                                         |
| `number`                              | `z.number()`                            |                                                         |
| positive integer (PKs, FKs)           | `z.number().int().positive()`           |                                                         |
| `boolean` (JSON / RHF)                | `z.boolean()`                           | See "Boolean & Coercion"                                |
| `boolean` (URL / raw form / env)      | `z.stringbool()`                        | See "Boolean & Coercion"                                |
| `1 \| 2 \| 3` (IntegerChoices)        | `z.literal([1, 2, 3])`                  | Single literal still uses `z.literal(1)`                |
| `'a' \| 'b'` (CharField choices)      | `z.enum(["a", "b"])`                    |                                                         |
| `Record<string, unknown>` (JSONField) | `z.record(z.string(), z.unknown())`     |                                                         |
| GenericForeignKey / unstructured      | `z.unknown()`                           | Prefer over `z.any()`                                   |
| `T \| null`                           | `zodType.nullable()`                    | Method, not `z.nullable(zodType)`                       |
| `T \| undefined` / `T?`               | `zodType.optional()`                    |                                                         |
| `T \| null \| undefined`              | `zodType.nullish()`                     |                                                         |
| `T[]`                                 | `z.array(zodType)`                      | M2M, nested lists                                       |
| nested object                         | `NestedSchema` or inline `z.object({})` | Only use `z.lazy(() => …)` for recursive types          |

---

## Boolean & Coercion Rules

**`z.coerce.boolean()` is banned.** It uses native `Boolean(input)`, which maps
the string `"false"` to `true` — a documented footgun. The Zod 4 docs recommend
`z.stringbool()` for any boolish string input.

| Origin                                | Use                                         | Why                              |
| ------------------------------------- | ------------------------------------------- | -------------------------------- |
| API JSON                              | `z.boolean()`                               | already a real boolean           |
| React Hook Form / controlled input    | `z.boolean()`                               | RHF emits real booleans          |
| HTML `<form>` raw `FormData`          | `z.stringbool()`                            | `"on"` / `"true"` mapped safely  |
| URL search param (nuqs raw)           | `z.stringbool()`                            | doc-standard truthy / falsy list |
| Env variable                          | `z.stringbool()`                            | environment string parsing       |
| Custom labels (e.g., `"yes"`/`"não"`) | `z.stringbool({ truthy: […], falsy: […] })` | explicit list overrides defaults |

`z.stringbool()` defaults (case-insensitive):

- truthy: `"true"`, `"1"`, `"yes"`, `"on"`, `"y"`, `"enabled"`
- falsy: `"false"`, `"0"`, `"no"`, `"off"`, `"n"`, `"disabled"`

```ts
// raw form / URL
is_active: z.stringbool(),

// JSON / RHF
is_active: z.boolean(),

// numbers / dates from URL — coerce is fine
total: z.coerce.number().positive(),
start_at: z.coerce.date(),
```

> Coerce **numbers and dates** is safe. **Never** coerce booleans.

---

## Discriminated Unions

When one field controls the shape of another, use `z.discriminatedUnion()`.
Extract a `Base` schema for shared fields, then `Base.extend({ … })` per arm.

```ts
const ScheduleBase = z.object({
  name: z.string().min(1),
  description: z.string().nullish(),
  is_active: z.boolean().optional(),
});

const CronConfigSchema = z.object({ cron: z.string().min(1) });
const IntervalConfigSchema = z.object({
  unit: z.enum(["minutes", "hours", "days"]),
  value: z.number().int().positive(),
});

export const CreateScheduleSchema = z.discriminatedUnion("schedule_type", [
  ScheduleBase.extend({
    schedule_type: z.literal(1),
    schedule_config: CronConfigSchema,
  }),
  ScheduleBase.extend({
    schedule_type: z.literal(2),
    schedule_config: IntervalConfigSchema,
  }),
]);

export type CreateScheduleDTO = z.infer<typeof CreateScheduleSchema>;
```

### Rules

1. Always prefer `z.discriminatedUnion()` over `z.union()` when a literal field
   discriminates. Use `z.union()` only when no single field discriminates.
2. Define each config variant as a **named sub-schema** before the union.
3. Extract a `Base` and `Base.extend({ … })` per arm — never duplicate shared
   fields across variants.
4. The discriminant must match a numeric/string union and is **required** in the
   Update schema (you cannot change the type of an existing record). See
   `references/advanced-patterns.md` §1 for the per-arm partial pattern.

---

## `z.any()` vs `z.unknown()`

| Situation                                          | Use                                     |
| -------------------------------------------------- | --------------------------------------- |
| `JSONField` with no predictable structure          | `z.record(z.string(), z.unknown())`     |
| GenericForeignKey / untyped payload                | `z.unknown()`                           |
| Extra serializer fields, validated keys still kept | `z.object({ … }).catchall(z.unknown())` |
| Extra fields preserved as-is, no validation        | `z.object({ … }).passthrough()`         |
| Source is explicitly `any` (third-party SDK)       | `z.any()`                               |

**Prefer `z.unknown()` over `z.any()`** — narrowing at the call site is safer.

---

## Update Schema Rule

`Update[Entity]Schema` is **always** derived from `Create[Entity]Schema`:

```ts
// pure Partial<CreateXDTO>
export const UpdateArticleSchema = CreateArticleSchema.partial();

// Partial<CreateXDTO> & { id: string }
export const UpdateArticleSchema = CreateArticleSchema.partial().extend({
  id: z.uuid(),
});
```

The schema must mirror whatever the DTO type says. If `UpdateXDTO` adds
required fields beyond `Partial<CreateXDTO>` (e.g., `id` so consumers can read
it back from the validated payload to build the PATCH URL), include them via
`.extend({ … })`.

**Exception** — `.partial()` does not work on `z.discriminatedUnion()`. Rebuild
the union with each arm partialed and the discriminant kept required. See
`references/advanced-patterns.md` §1.

---

## Barrel Export

Schemas are runtime values. The barrel re-exports schemas only — DTO types live
in the `types/` barrel.

```ts
// domains/<context>/schemas/index.ts
export { CreateArticleSchema, UpdateArticleSchema } from "./article-schema";
export { CreateScheduleSchema, UpdateScheduleSchema } from "./schedule-schema";
```

```ts
// consumer file
import type { CreateArticleDTO } from "@/domains/articles/types";
import { CreateArticleSchema } from "@/domains/articles/schemas";
```

When adding a new schema file, always show the updated `index.ts`.

---

## Audit Mode

Trigger when the user asks to "audit", "check", "review", "find drift in", or
"validate" existing schemas against the Zod 4 patterns above. Audit Mode is
read-only — it produces a report, never edits files.

### Workflow

1. **Locate** — `find apps/web/src/domains -name "*-schema.ts"` (or scope to a
   bounded context if the user names one).
2. **Inspect** each schema file against the checklist below.
3. **Report** findings grouped by file and tagged with severity.
4. **Stop** — do not change anything until the user approves the plan.

### Drift Checklist (severity-tagged)

**🔴 Critical — wrong runtime behavior**

| Symptom                                                          | Fix                                                     |
| ---------------------------------------------------------------- | ------------------------------------------------------- |
| `z.coerce.boolean()` anywhere                                    | `z.stringbool()` (raw form/URL) or `z.boolean()` (JSON) |
| `Update[Entity]Schema = z.object({ … })` redefining fields       | `CreateXSchema.partial()`                               |
| `Response[Entity]Schema` validating API responses at runtime     | Remove — Response types come from `django-to-typescript`           |
| Discriminated union arm where discriminant is not a literal/enum | Use `z.literal(…)` on the discriminant field            |
| `.partial()` applied directly on `z.discriminatedUnion(...)`     | Rebuild union per arm (see advanced-patterns §1)        |

**🟡 Drift from Zod 4 — compile-time still works**

| Symptom                                                | Fix                                                 |
| ------------------------------------------------------ | --------------------------------------------------- |
| `z.string().uuid()` / `.email()` / `.url()` / `.ip()`  | `z.uuid()` / `z.email()` / `z.url()` / `z.ipv4()`   |
| `z.string().datetime({ offset: true })`                | `z.iso.datetime({ offset: true })`                  |
| `z.union([z.literal(1), z.literal(2), …])`             | `z.literal([1, 2, …])`                              |
| `z.nullable(zodType)` (function form)                  | `zodType.nullable()` (method)                       |
| `satisfies z.ZodType<DTO>` on a schema                 | Remove (schema mirrors DTO; no static binding)      |
| `: z.ZodType<X>` annotation on a schema const          | Remove the annotation (let Zod infer)               |
| `export type CreateXDTO = z.infer<…>` from schema file | Remove — DTO is owned by `types/`                   |
| Schema fields drift from DTO type                      | Re-mirror the schema from `types/<entity>-types.ts` |

**🟢 Hygiene**

| Symptom                                                                    | Fix                                     |
| -------------------------------------------------------------------------- | --------------------------------------- |
| Discriminated union arms duplicate shared fields                           | Extract `Base` and `Base.extend({ … })` |
| Missing or stale `schemas/index.ts` barrel                                 | Re-export Schema + inferred DTO types   |
| File name not `[entity]-schema.ts` / variable not `[Action][Entity]Schema` | Rename to convention                    |
| Schema file imports `*-types.ts` only for `satisfies`                      | Drop the import                         |

### Audit Report Format

Group by file. Cite line numbers. Use the severity icons.

```
## domains/orders/schemas/order-schema.ts

🔴 Critical
- L23: `z.coerce.boolean()` on `is_active` → use `z.stringbool()` (raw form context)
- L41: `UpdateOrderSchema = z.object({ … })` redefines fields → use `CreateOrderSchema.partial()`

🟡 Zod 4 drift
- L12: `z.string().uuid()` → `z.uuid()`
- L18: `satisfies z.ZodType<CreateOrderDTO>` → remove + `export type CreateOrderDTO = z.infer<typeof CreateOrderSchema>`
- L7:  `import type { CreateOrderDTO } from "../types"` → no longer needed once inferred

🟢 Hygiene
- L35-58: Discriminated union arms duplicate `name`, `description` → extract `OrderBase`
```

Close the report with: a count by severity, and a one-line recommendation
("safe one-shot fix" / "needs cross-file plan" / "blocked on backend confirm").

---

## Refactor Plan Mode

Trigger when the user asks to "plan refactor", "migrate the schema", "update
schema and consumers", or follows up on an audit with "let's fix it". Plan Mode
produces a written plan; it executes only after user approval.

### Workflow: audit → schema plan → cross-file impact → execution order

**Step 1 — Schema-level plan (per file)**

For each schema flagged in the audit, list exact line-level replacements:

```
domains/orders/schemas/order-schema.ts
  - replace `z.string().uuid()` (L12) with `z.uuid()`
  - replace `z.coerce.boolean()` on `is_active` (L23) with `z.stringbool()`
  - remove `satisfies z.ZodType<CreateOrderDTO>` (L18)
  - remove import of `CreateOrderDTO` from "../types" (L7)
  - add `export type CreateOrderDTO = z.infer<typeof CreateOrderSchema>` (after schema)
  - rebuild Update schema: `UpdateOrderSchema = CreateOrderSchema.partial()` (L41)
  - rebuild discriminated union with `OrderBase.extend(...)` per arm (L35-58)
```

**Step 2 — Cross-file impact (per schema changed)**

Run `grep -r` to find every importer. Classify each:

| Importer pattern                                       | Action                                                                                                                       |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------- |
| `import type { CreateXDTO } from "*/types"`            | No change — DTO continues to live in `types/`                                                                                |
| `import { CreateXSchema } from "*/schemas/...-schema"` | Optionally repoint to barrel (`from "../schemas"`) for consistency                                                           |
| `zodResolver(CreateXSchema)` in RHF                    | Verify still compiles after refactor; if schema gained a transform, may need `z.input<typeof Schema>` for `useForm` defaults |
| `useForm<CreateXDTO>(…)` form values type              | Keep `CreateXDTO`; if schema gained a transform, switch to `z.input<typeof CreateXSchema>`                                   |
| `mutation.mutate(values: CreateXDTO)`                  | Keep `CreateXDTO`; if schema has transforms, parsed payload is `z.output<typeof Schema>`                                     |
| nuqs / search-param parsers depending on schema        | Rebuild parser using `z.stringbool()` / `z.coerce.*` per Zod 4 mapping                                                       |
| Schema-field drift from DTO                            | Update the **schema** to match the DTO type; if the DTO itself is wrong, fix it via `django-to-typescript` first                        |

**Step 3 — Execution order**

1. Apply all schema-file changes for the entity in one commit.
2. Update the entity's `schemas/index.ts` barrel.
3. Run `cd apps/web && yarn tsc --noEmit` — let TypeScript surface every broken
   import.
4. Repoint each broken import flagged by `tsc`. Prefer one entity at a time.
5. Run `yarn lint` and `yarn test`.
6. If the refactor swapped `z.coerce.boolean()` → `z.stringbool()`, **manually
   exercise** the affected form/URL flow in the dev server — tests can pass
   while a checkbox silently changes semantics.
7. If a schema gained a transform (input ≠ output), audit React Hook Form
   defaults and submit handlers in the importing components.

### Plan Output Format

Open the plan with a one-line scope statement, then the three steps. End with
explicit "Acceptance" criteria the user can check off:

```
Scope: refactor `orders` schemas to Zod 4 patterns + repoint 6 importing files.

Step 1 — Schema changes
  - domains/orders/schemas/order-schema.ts (8 replacements)
  - domains/orders/schemas/index.ts (re-export inferred DTOs)

Step 2 — Importers (from grep)
  - apps/web/src/app/(authenticated)/orders/new/page.tsx — type-only repoint
  - apps/web/src/components/forms/order-form.tsx — RHF resolver still compatible
  - apps/web/src/lib/queries/orders.ts — repoint DTO import
  - … (3 more)

Step 3 — Execution
  1. Apply schema changes.  2. Run `tsc --noEmit`.  3. Repoint imports.
  4. `yarn lint` + `yarn test`.  5. Manual smoke on order create form.

Acceptance
  ☐ tsc --noEmit clean
  ☐ yarn lint clean
  ☐ yarn test passes
  ☐ POST /orders/ submits with `is_active: boolean` correctly
  ☐ Order edit form pre-fills correctly
```

Wait for the user to approve before executing.

---

## Reference Files

- `examples/examples.md` — worked examples: primitives, numeric unions,
  nullable/optional, `z.stringbool()` for forms/URL, JSONField, discriminated
  union (Schedule), shared field schemas.
- `references/advanced-patterns.md` — partial on discriminated unions, nested
  unions, runtime narrowing, pipelines, cross-field `superRefine`, codecs for
  bidirectional transforms, divergent input/output forms.
