# Phase 16: Webhooks and request logs

**Depends on:** 13, 15. **Design:** [`to-be.md`](../design/to-be.md) decision 26, sections 8.4 and 12.5.

## Goal

Tell merchants what happened to their payments through signed, retried webhooks, and show them the API calls their systems made.

## In scope

1. **Context `libs/webhooks`:** endpoints (URL, event types, status), deliveries, dispatcher. New Go module in `go.work` and the root `PACKAGES`, with a `webhooks-isolation` depguard rule in `configs/go/golangci.yml`.
2. **Dispatcher:** consumes the payment and checkout topics with inbox dedupe, creates one delivery per matching endpoint, sends it, and schedules retries in the database (`next_attempt_at`) with the backoff 1 min, 5 min, 30 min, 2 h, 6 h, 24 h, then marks the delivery failed. Runs in the phase 12 worker.
3. **Signature:** `Iris-Signature: t=<unix>,v1=<hex HMAC-SHA256(secret, "<t>.<raw body>")>`; event body with `id`, `type`, `created`, `data`. The endpoint secret is shown once at creation and stored wrapped by the key manager (apps/api gets its own OpenBao policy for a webhook key).
4. **SSRF protection:** endpoint URLs must be `https`, resolve to public addresses only, and are re-checked at send time (DNS rebinding). A dev flag allows `http://localhost` and Compose service names, off by default outside local `.env`.
5. **API and dashboard:** endpoint CRUD, delivery list with status, response code and latency, redeliver, and "send test event".
6. **Request log:** middleware writes `api_request_logs` (to-be section 8.4) for API-key requests, metadata only; a retention job deletes rows older than 30 days; dashboard page with filters (status, route, key).
7. **Receiver guide:** README section with signature verification snippets in Go and Node, and the 5-minute timestamp tolerance.

## Out of scope

Ledger. Email notifications. Webhook endpoint per connected account.

## Guidelines

- Delivery timeout 10 s; any 2xx is success; redirects are not followed.
- A slow or failing endpoint must not delay other merchants' deliveries (bounded concurrency per endpoint).
- Request and response bodies are never logged, in webhooks or in the request log.

## Acceptance criteria

1. A captured payment produces one delivery to each subscribed endpoint, with a signature that the documented snippet verifies.
2. An endpoint that fails twice then succeeds ends `delivered` on the third attempt, at the scheduled times (fake clock).
3. Endpoints pointing at `127.0.0.1`, `169.254.169.254`, private ranges or a hostname that resolves to them are rejected (dev flag off).
4. Redeliver sends the same event id with a new signature timestamp.
5. Request-log rows exist for API-key calls, carry no body, and are deleted after 30 days by the retention job.

## Docs to update

`AGENTS.md` (webhooks context), README (webhooks guide), `docs/security.md` (SSRF control).
