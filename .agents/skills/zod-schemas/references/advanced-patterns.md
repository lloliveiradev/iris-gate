# Advanced Zod 4 Patterns

All patterns assume the canonical flow: **DTO types live in `types/`** (owned
by `django-to-typescript`); the schema in `schemas/` mirrors the DTO and acts as the
runtime validator. No `satisfies z.ZodType<…>`. No `z.infer` / `z.input` /
`z.output` exports from the schema file — use them inline at the consuming
code if needed.

---

## 1. Partial on Discriminated Unions

`.partial()` only works on `z.object()` — it does **not** apply to
`z.discriminatedUnion()`. To produce an Update schema for a discriminated entity,
rebuild the union with each arm partialed and the discriminant kept required.

```ts
// ❌ does not work
export const UpdateScheduleSchema = CreateScheduleSchema.partial();

// ✅ correct — extract base, partial it, rebuild union per arm
const ScheduleBase = z.object({
  name: z.string().min(1),
  is_active: z.boolean().optional(),
});
const ScheduleBasePartial = ScheduleBase.partial();

export const UpdateScheduleSchema = z.discriminatedUnion("schedule_type", [
  ScheduleBasePartial.extend({
    schedule_type: z.literal(1), // required on PATCH
    schedule_config: CronConfigSchema.partial().optional(),
  }),
  ScheduleBasePartial.extend({
    schedule_type: z.literal(2),
    schedule_config: IntervalConfigSchema.partial().optional(),
  }),
]);
```

**Rule:** the discriminant (`schedule_type`, `type`, etc.) is always required in
the Update schema — you cannot change the type of an existing record.

---

## 2. Nested Discriminated Unions

When a discriminated union arm contains another discriminated field, build the
inner union as a standalone schema and reference it.

```ts
const EmailNotificationSchema = z.object({
  channel: z.literal("email"),
  config: z.object({
    to: z.email(),
    template: z.string(),
  }),
});

const SlackNotificationSchema = z.object({
  channel: z.literal("slack"),
  config: z.object({
    webhook_url: z.url(),
    channel_name: z.string(),
  }),
});

// inner union
const NotificationSchema = z.discriminatedUnion("channel", [
  EmailNotificationSchema,
  SlackNotificationSchema,
]);

// outer union — uses the inner union as a field
export const JobSchema = z.discriminatedUnion("type", [
  z.object({ type: z.literal("notification"), payload: NotificationSchema }),
  z.object({ type: z.literal("export"), payload: ExportConfigSchema }),
]);
```

---

## 3. Runtime Narrowing After Parse

After parsing a discriminated union, narrow by the discriminant. TypeScript
follows the literal automatically.

```ts
const result = CreateScheduleSchema.parse(input);

if (result.schedule_type === 1) {
  result.schedule_config.cron; // typed as CronConfig
} else if (result.schedule_type === 2) {
  result.schedule_config.unit; // typed as IntervalConfig
}
```

---

## 4. Pipelines for String → Type

Use `.pipe()` when a value arrives as a string but must be re-validated as
another shape downstream.

```ts
// URL string → number → IntegerChoices literal
const status = z.coerce.number().pipe(z.literal([1, 2, 3, 4, 5]));

// URL string → ISO datetime → JS Date
const dateParam = z.iso.datetime({ offset: true }).pipe(z.coerce.date());
```

Never pipe data that arrives from an API JSON response — it is already typed.

For bidirectional transforms (form value ↔ stored value), prefer **codecs**
(`z.codec(...)` — Zod 4) over chained pipes; they preserve both directions.

---

## 5. Reusable Field Schemas

Extract frequently reused fields into named constants. Keep them in a private
`shared-schemas.ts` next to the schemas; do not re-export from the barrel.

```ts
// shared-schemas.ts
import { z } from "zod";

export const idField = z.uuid();
export const isoDateField = z.iso.datetime({ offset: true });
export const positiveInt = z.number().int().positive();
export const slugField = z.string().regex(/^[-a-zA-Z0-9_]+$/);
```

```ts
// article-schema.ts
import { z } from "zod";
import { positiveInt, isoDateField } from "./shared-schemas";

export const CreateArticleSchema = z.object({
  title: z.string().min(1).max(200),
  author: positiveInt,
  scheduled: isoDateField.optional(),
});
```

---

## 6. Catching Extra Fields from DRF

When a DRF serializer may return undocumented extra fields (dynamic annotations,
nested expansions), choose explicitly:

```ts
// passthrough — extra fields kept as-is, no validation on them
const LooseArticleSchema = z
  .object({
    id: z.uuid(),
    title: z.string(),
  })
  .passthrough();

// catchall — extra fields validated as a specific (loose) type
const StrictArticleSchema = z
  .object({
    id: z.uuid(),
    title: z.string(),
  })
  .catchall(z.unknown());
```

Use `passthrough()` when extras will arrive and you don't care about them.
Use `catchall(z.unknown())` to explicitly mark "extras exist, not yet typed".

---

## 7. Cross-Field Validation with `superRefine`

When one field's validity depends on another, use `superRefine` to add issues
on a specific path.

```ts
export const CreateBookingSchema = z
  .object({
    start_at: z.iso.datetime({ offset: true }),
    end_at: z.iso.datetime({ offset: true }).nullable(),
  })
  .superRefine((data, ctx) => {
    if (data.end_at && data.end_at <= data.start_at) {
      ctx.addIssue({
        code: "custom",
        path: ["end_at"],
        message: "end_at must be after start_at",
      });
    }
  });
```

`superRefine` runs after the base object schema validates, so individual field
errors come first and cross-field errors second.

---

## 8. Form Schemas with Divergent Input/Output

When a schema uses `z.stringbool()`, `z.coerce.*`, or `.transform()`, the input
type (form values) and output type (parsed payload) diverge. The schema file
exports only the schema; consumers use `z.input` / `z.output` **inline**.

```ts
// domains/orders/schemas/order-form-schema.ts
import { z } from "zod";

export const OrderFormSchema = z.object({
  total: z.coerce.number().positive(), // input: string | number → output: number
  is_active: z.stringbool(), // input: string → output: boolean
  status: z.literal([1, 2, 3, 4, 5]),
});
```

```ts
// in the form component
import { OrderFormSchema } from "../schemas";

type FormValues = z.input<typeof OrderFormSchema>; // form raw values
type SubmitPayload = z.output<typeof OrderFormSchema>; // parsed, ready to POST

const form = useForm<FormValues>({ resolver: zodResolver(OrderFormSchema) });
```

Libraries that derive form value types from the schema (e.g., `react-hook-form`
with `@hookform/resolvers/zod`) read `z.input<>`. Use `z.output<>` for the
value you actually submit to the backend.
