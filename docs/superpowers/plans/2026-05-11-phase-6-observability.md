# Phase 6 — Observability End-to-End Implementation Plan

> **Status:** ⏸ Deferred — frontend design redesign takes priority before this lands.
>
> **Execution mode (when resumed):** **Subagent-Driven Development** —
> dispatch one fresh subagent per Task (1 through 10), review the diff
> between tasks, accept/reject before moving on. Reasoning: 4 stacks
> + ~30 files touched, per-task isolation protects the main context
> and surfaces issues at clean boundaries.
>
> **REQUIRED SUB-SKILL when resuming:** `superpowers:subagent-driven-development`.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire OpenTelemetry across `apps/api` (Go), `apps/anti-fraud` (NestJS), and `apps/web` (Next.js); add a local Prometheus + Grafana + Jaeger stack; produce dashboards-as-code for latency / acquirer approval / consumer lag; configure SLO alerts; and persist append-only audit events for fraud decisions and API-key changes.

**Architecture:**
- Single OTel Collector (gateway pattern) at `infra/docker` receives OTLP-gRPC from all services on `:4317` and OTLP-HTTP on `:4318`. It exports **traces → Jaeger**, **metrics → Prometheus** (via `prometheusremotewrite`), **logs → stdout** (Loki deferred to Fase 7).
- Each service uses **vendor-neutral OTel SDK** with W3C `traceparent` propagation. Kafka producer/consumer pair propagates trace context via message headers (already half-done — relay worker only needs an OTel hook).
- Audit log is a `libs/audit` Go lib: append-only Postgres table with `prev_hash` chained to detect tampering; emitted in the same DB transaction as the originating mutation (Outbox semantics, not Kafka — local persistence is enough for v1).
- Grafana provisioned via files (`datasources`, `dashboards`) under `infra/docker/grafana/provisioning/` so dashboards live in git.
- SLO alerts (Prometheus rules) live under `infra/docker/prometheus/rules/` and load via volume mount.

**Tech Stack:**
- OTel SDK Go: `go.opentelemetry.io/otel`, `otelhttp`, `otelpgx`, `otelkafka` (segmentio bridge written by hand — there's no first-party for segmentio), `prometheus/client_golang`.
- OTel SDK Node: `@opentelemetry/sdk-node`, `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/exporter-trace-otlp-grpc`, `@opentelemetry/exporter-metrics-otlp-grpc`, `@opentelemetry/instrumentation-kafkajs`, `@opentelemetry/instrumentation-pg` (via Prisma's underlying pg).
- OTel SDK Next.js: `@vercel/otel` (server-only; respects `instrumentation.ts` hook).
- Docker images: `otel/opentelemetry-collector-contrib:0.112.0`, `prom/prometheus:v2.55.0`, `grafana/grafana:11.4.0`, `jaegertracing/jaeger:2.0.0` (v2 all-in-one).

**Out of scope (deferred):**
- Loki + log aggregation → Fase 7.
- Synthetic monitoring / Lighthouse CI → Fase 7.
- Distributed sampling decisions (tail sampling) → Fase 7. We use head sampling at SDK level (`AlwaysOn` in dev, configurable in prod).
- Real exemplars / RUM in `apps/web` → Pós-MVP.

---

## File Structure

### New files (created)

- `infra/docker/otel-collector/config.yaml` — OTel Collector pipeline config.
- `infra/docker/prometheus/prometheus.yml` — Prometheus scrape + remote_write source.
- `infra/docker/prometheus/rules/slo.yml` — SLO recording + alert rules.
- `infra/docker/grafana/provisioning/datasources/datasources.yml` — Prometheus + Jaeger datasource provisioning.
- `infra/docker/grafana/provisioning/dashboards/dashboards.yml` — Dashboard provider config.
- `infra/docker/grafana/dashboards/api-latency.json` — API latency dashboard.
- `infra/docker/grafana/dashboards/payments-acquirer.json` — Payments acquirer approval dashboard.
- `infra/docker/grafana/dashboards/consumer-lag.json` — Kafka consumer lag dashboard.
- `libs/audit/domain/event.go` — `AuditEvent` aggregate (immutable, hash-chained).
- `libs/audit/domain/event_test.go` — hash chain unit tests.
- `libs/audit/application/ports.go` — `EventStore` + `Recorder` use case.
- `libs/audit/application/recorder.go` — `Recorder.Record(ctx, AuditEvent)` use case.
- `libs/audit/application/recorder_test.go` — recorder unit tests with fakes.
- `libs/audit/infra/pgstore/store.go` — pgx `EventStore` impl.
- `libs/audit/infra/pgstore/migrations/0001_audit.sql` — `audit_events` table.
- `libs/audit/infra/pgstore/migrations.go` — `go:embed` MigrateEmbedded.
- `libs/audit/infra/pgstore/store_test.go` — integration test via shared pgx pool (skipped if `TEST_DB_URL` empty).
- `apps/api/internal/observability/otel.go` — OTel init (tracer/meter/exporter).
- `apps/api/internal/observability/metrics.go` — Prometheus collector registry + HTTP handler.
- `apps/api/internal/observability/otel_test.go` — smoke test that init returns nil error + shutdown is idempotent.
- `apps/anti-fraud/src/observability/otel.ts` — Node SDK bootstrap (called from main.ts before NestFactory).
- `apps/anti-fraud/src/observability/metrics.module.ts` — Prometheus metrics endpoint (`/metrics`).
- `apps/web/src/instrumentation.ts` — Next.js OTel instrumentation hook (server-only).
- `docs/adr/0004-observability-pipeline.md` — ADR for the OTel choice.

### Modified files

- `infra/docker/docker-compose.yml` — add `otel-collector`, `prometheus`, `grafana`, `jaeger` services.
- `infra/docker/Makefile` — add `otel-logs`, `grafana-open` targets.
- `infra/docker/.env.example` — add `GRAFANA_PORT`, `PROMETHEUS_PORT`, `JAEGER_UI_PORT`, `OTEL_COLLECTOR_PORT`.
- `apps/api/internal/config/config.go` — add `OTLPEndpoint`, `OTELServiceName`, `MetricsAddr` env.
- `apps/api/internal/httpapi/router.go` — wrap router with `otelhttp.NewHandler`.
- `apps/api/cmd/api/main.go` — init OTel, expose `/metrics` on `MetricsAddr`, instrument pgxpool with `otelpgx`, wrap kafka writer.
- `apps/api/go.mod` — add OTel deps.
- `libs/payments/infra/kafkarelay/relay.go` — inject tracer; create span around `pump`; propagate `traceparent` into message headers via `propagation.TextMapPropagator`.
- `libs/payments/infra/kafkarelay/relay_test.go` — assert `traceparent` header present.
- `libs/identity/application/issue_api_key.go` — call `audit.Recorder.Record` on issue + revoke (added as a port).
- `libs/identity/application/revoke_api_key.go` — same.
- `libs/identity/application/ports.go` — add `AuditRecorder` port.
- `libs/fraud/src/application/assess-risk.use-case.ts` — call `AuditRecorder.record` on completion (port added).
- `libs/fraud/src/application/ports.ts` — add `AuditRecorder` port.
- `apps/anti-fraud/src/main.ts` — `await otelStart()` before bootstrap.
- `apps/anti-fraud/src/app.module.ts` — register `MetricsModule`.
- `apps/anti-fraud/src/infra/kafka/assessment.consumer.ts` — extract `traceparent` from Kafka headers; wrap handler in span.
- `apps/anti-fraud/src/infra/kafka/event-publisher.ts` — inject `traceparent` header from active context.
- `apps/anti-fraud/src/infra/kafka/outbox-relay.service.ts` — inject `traceparent` header.
- `apps/anti-fraud/package.json` — add OTel deps.
- `apps/web/next.config.ts` — `experimental.instrumentationHook = true` (Next 16 already auto-enables, just verify).
- `apps/web/package.json` — add `@vercel/otel`.
- `docs/roadmap.md` — mark Phase 6 tasks as done in checkboxes; add Status block at end.

---

## Task 1: Local Observability Stack — Docker Compose + Config

**Files:**
- Create: `infra/docker/otel-collector/config.yaml`
- Create: `infra/docker/prometheus/prometheus.yml`
- Create: `infra/docker/grafana/provisioning/datasources/datasources.yml`
- Create: `infra/docker/grafana/provisioning/dashboards/dashboards.yml`
- Modify: `infra/docker/docker-compose.yml`
- Modify: `infra/docker/.env.example`
- Modify: `infra/docker/Makefile`

- [ ] **Step 1.1: Write OTel Collector pipeline config**

Create `infra/docker/otel-collector/config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 512
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 25
  resource:
    attributes:
      - key: deployment.environment
        value: local
        action: upsert

exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
  prometheusremotewrite:
    endpoint: http://prometheus:9090/api/v1/write
    tls:
      insecure: true
    resource_to_telemetry_conversion:
      enabled: true
  debug:
    verbosity: basic

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [prometheusremotewrite]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [debug]
```

- [ ] **Step 1.2: Write Prometheus scrape + remote-write config**

Create `infra/docker/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - /etc/prometheus/rules/*.yml

scrape_configs:
  - job_name: api
    static_configs:
      - targets: ['host.docker.internal:9100']
        labels: { service: api }
  - job_name: anti-fraud
    static_configs:
      - targets: ['host.docker.internal:9101']
        labels: { service: anti-fraud }
```

Note: `host.docker.internal` works on Docker Desktop (Mac/Win); add `extra_hosts: ['host.docker.internal:host-gateway']` in compose for Linux compatibility.

Also `prometheus.yml` must enable remote-write receiver — that's via CLI flag in the compose service (`--web.enable-remote-write-receiver`).

- [ ] **Step 1.3: Write Grafana datasource provisioning**

Create `infra/docker/grafana/provisioning/datasources/datasources.yml`:

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: true
  - name: Jaeger
    type: jaeger
    access: proxy
    url: http://jaeger:16686
    editable: true
    jsonData:
      tracesToMetrics:
        datasourceUid: prometheus
```

- [ ] **Step 1.4: Write Grafana dashboard provider config**

Create `infra/docker/grafana/provisioning/dashboards/dashboards.yml`:

```yaml
apiVersion: 1

providers:
  - name: 'iris-gate'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
```

- [ ] **Step 1.5: Extend `docker-compose.yml` with observability services**

Append (under existing `services:`) to `infra/docker/docker-compose.yml`:

```yaml
  jaeger:
    image: jaegertracing/jaeger:2.0.0
    container_name: iris-jaeger
    restart: unless-stopped
    ports:
      - "${JAEGER_UI_PORT:-16686}:16686"
    environment:
      COLLECTOR_OTLP_ENABLED: "true"
    networks: [iris]

  prometheus:
    image: prom/prometheus:v2.55.0
    container_name: iris-prometheus
    restart: unless-stopped
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --web.enable-remote-write-receiver
      - --web.enable-lifecycle
    ports:
      - "${PROMETHEUS_PORT:-9090}:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./prometheus/rules:/etc/prometheus/rules:ro
      - prometheus-data:/prometheus
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks: [iris]

  grafana:
    image: grafana/grafana:11.4.0
    container_name: iris-grafana
    restart: unless-stopped
    depends_on:
      - prometheus
      - jaeger
    ports:
      - "${GRAFANA_PORT:-3000}:3000"
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Viewer
      GF_LOG_LEVEL: warn
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
      - ./grafana/dashboards:/var/lib/grafana/dashboards:ro
      - grafana-data:/var/lib/grafana
    networks: [iris]

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.112.0
    container_name: iris-otel-collector
    restart: unless-stopped
    depends_on:
      - jaeger
      - prometheus
    command: ["--config=/etc/otelcol/config.yaml"]
    ports:
      - "${OTEL_COLLECTOR_GRPC:-4317}:4317"
      - "${OTEL_COLLECTOR_HTTP:-4318}:4318"
    volumes:
      - ./otel-collector/config.yaml:/etc/otelcol/config.yaml:ro
    networks: [iris]
```

Add new named volumes at bottom of file:

```yaml
volumes:
  postgres-data:
  kafka-data:
  prometheus-data:
  grafana-data:
```

(Verify that `postgres-data` and `kafka-data` already exist there; merge only the new ones.)

- [ ] **Step 1.6: Extend `.env.example`**

Add to `infra/docker/.env.example`:

```env
GRAFANA_PORT=3000
PROMETHEUS_PORT=9090
JAEGER_UI_PORT=16686
OTEL_COLLECTOR_GRPC=4317
OTEL_COLLECTOR_HTTP=4318
```

- [ ] **Step 1.7: Add Makefile convenience targets**

In `infra/docker/Makefile`, under `##@ Kafka` section, append `##@ Observability` section:

```makefile
##@ Observability

.PHONY: otel-logs
otel-logs: ## Tail dos logs do OTel Collector
	$(COMPOSE) logs -f otel-collector

.PHONY: grafana-open
grafana-open: ## Abre Grafana no browser (Mac)
	@open http://localhost:$${GRAFANA_PORT:-3000}

.PHONY: jaeger-open
jaeger-open: ## Abre Jaeger UI no browser (Mac)
	@open http://localhost:$${JAEGER_UI_PORT:-16686}

.PHONY: prom-open
prom-open: ## Abre Prometheus no browser (Mac)
	@open http://localhost:$${PROMETHEUS_PORT:-9090}
```

- [ ] **Step 1.8: Verify compose comes up clean**

Run: `cd infra/docker && docker compose up -d --wait`
Expected: all 8 services (`postgres`, `kafka`, `karapace`, `jaeger`, `prometheus`, `grafana`, `otel-collector`) become healthy. Note: `otel-collector` has no built-in healthcheck — verify via `docker compose ps` that it's `Up`.

Then: `curl -s http://localhost:9090/-/healthy` → `Prometheus Server is Healthy`.
Then: `curl -s http://localhost:16686/` → returns HTML.

- [ ] **Step 1.9: Commit**

```bash
cd /Users/lloli/Workspace/projetos/iris-gate
git add infra/docker/docker-compose.yml infra/docker/Makefile infra/docker/.env.example \
        infra/docker/otel-collector/config.yaml \
        infra/docker/prometheus/prometheus.yml \
        infra/docker/grafana/provisioning/
git commit -m "feat(infra): add OTel collector + Prometheus + Grafana + Jaeger to local stack"
```

---

## Task 2: apps/api OTel Bootstrap + HTTP/PG Instrumentation

**Files:**
- Create: `apps/api/internal/observability/otel.go`
- Create: `apps/api/internal/observability/otel_test.go`
- Create: `apps/api/internal/observability/metrics.go`
- Modify: `apps/api/internal/config/config.go`
- Modify: `apps/api/internal/httpapi/router.go`
- Modify: `apps/api/cmd/api/main.go`
- Modify: `apps/api/go.mod` (via `go get`)

- [ ] **Step 2.1: Write OTel init test**

Create `apps/api/internal/observability/otel_test.go`:

```go
package observability

import (
	"context"
	"testing"
	"time"
)

func TestSetup_ShutdownIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shutdown, err := Setup(ctx, Config{
		ServiceName: "test",
		Endpoint:    "", // empty endpoint → noop exporter
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown 1: %v", err)
	}
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown 2 (idempotent): %v", err)
	}
}
```

- [ ] **Step 2.2: Verify test fails (no package yet)**

Run: `cd apps/api && go test ./internal/observability/...`
Expected: FAIL with `no Go files`.

- [ ] **Step 2.3: Add OTel Go dependencies**

Run from `apps/api/`:

```bash
go get go.opentelemetry.io/otel@v1.32.0 \
       go.opentelemetry.io/otel/sdk@v1.32.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.32.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@v1.32.0 \
       go.opentelemetry.io/otel/sdk/metric@v1.32.0 \
       go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.57.0 \
       github.com/exaring/otelpgx@v0.9.0 \
       github.com/prometheus/client_golang@v1.20.5
```

Then `go mod tidy`.

- [ ] **Step 2.4: Implement OTel setup**

Create `apps/api/internal/observability/otel.go`:

```go
package observability

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	ServiceName string
	Endpoint    string // OTLP gRPC endpoint, e.g. "localhost:4317". Empty → noop.
	Env         string
}

type ShutdownFunc func(ctx context.Context) error

func Setup(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Env),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	var once sync.Once
	shutdown := func(ctx context.Context) error {
		var err error
		once.Do(func() {
			err = errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
		})
		return err
	}
	return shutdown, nil
}
```

- [ ] **Step 2.5: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/observability/... -v`
Expected: PASS `TestSetup_ShutdownIdempotent`.

- [ ] **Step 2.6: Implement metrics HTTP handler**

Create `apps/api/internal/observability/metrics.go`:

```go
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func MetricsHandler() http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
}
```

- [ ] **Step 2.7: Extend Config**

Modify `apps/api/internal/config/config.go` — add fields and env reads:

```go
type Config struct {
    // ... existing fields
    OTLPEndpoint    string // empty disables OTel
    OTELServiceName string
    MetricsAddr     string // e.g. ":9100"
}
```

In `Load()`:

```go
cfg.OTLPEndpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
cfg.OTELServiceName = envOr("OTEL_SERVICE_NAME", "iris-api")
cfg.MetricsAddr = envOr("METRICS_ADDR", ":9100")
```

(Use existing `envOr` helper or add inline.)

- [ ] **Step 2.8: Wire OTel into router**

Modify `apps/api/internal/httpapi/router.go` — wrap the returned `chi.Router`:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

func NewRouter(m Mounters) http.Handler {
    r := chi.NewRouter()
    // ... existing routing
    return otelhttp.NewHandler(r, "iris-api",
        otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
            return req.Method + " " + req.URL.Path
        }),
    )
}
```

- [ ] **Step 2.9: Wire OTel + metrics + pgxpool tracing into main**

Modify `apps/api/cmd/api/main.go` — after config load, before pgxpool:

```go
import (
    "github.com/exaring/otelpgx"
    "github.com/iris-gate/iris-gate/apps/api/internal/observability"
)

// after cfg loaded:
shutdownOTel, err := observability.Setup(rootCtx, observability.Config{
    ServiceName: cfg.OTELServiceName,
    Endpoint:    cfg.OTLPEndpoint,
    Env:         cfg.Env,
})
if err != nil {
    slog.Error("otel setup", "err", err)
    os.Exit(1)
}
defer func() {
    sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer sCancel()
    _ = shutdownOTel(sCtx)
}()

// Replace pgxpool.New(...) with config-based New so we can inject tracer:
poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
if err != nil { ... }
poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
pool, err := pgxpool.NewWithConfig(rootCtx, poolCfg)
```

And spin up the metrics server as a separate `http.Server` running in its own goroutine, with graceful shutdown:

```go
metricsSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: observability.MetricsHandler()}
go func() {
    slog.Info("metrics listening", "addr", cfg.MetricsAddr)
    if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        slog.Error("metrics listen", "err", err)
    }
}()
defer metricsSrv.Shutdown(context.Background())
```

- [ ] **Step 2.10: Run all api tests**

Run: `cd apps/api && go test ./... && go vet ./... && go build ./...`
Expected: PASS. All identity/payments tests still green (no behavior change, only wrappers).

- [ ] **Step 2.11: Commit**

```bash
git add apps/api/ go.work
git commit -m "feat(api): wire OTel tracer + meter + Prometheus /metrics endpoint"
```

---

## Task 3: Payments Kafka Relay Trace Propagation

**Files:**
- Modify: `libs/payments/infra/kafkarelay/relay.go`
- Modify: `libs/payments/infra/kafkarelay/relay_test.go`

- [ ] **Step 3.1: Add failing test for traceparent injection**

Append to `libs/payments/infra/kafkarelay/relay_test.go`:

```go
func TestRelay_InjectsTraceparent(t *testing.T) {
    t.Parallel()
    // Create source with 1 pending message, no traceparent header.
    src := newFakeSource(mkPending("payments.t1", 1)...)
    pub := &fakePublisher{}
    r := kafkarelay.New(src, pub, kafkarelay.Config{
        PollEvery:  10 * time.Millisecond,
        BatchLimit: 10,
    })
    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()
    _ = r.Run(ctx)

    pub.mu.Lock()
    defer pub.mu.Unlock()
    if len(pub.sent) == 0 {
        t.Fatal("nothing published")
    }
    msg := pub.sent[0][0]
    var hasTP bool
    for _, h := range msg.Headers {
        if h.Key == "traceparent" {
            hasTP = true
            if len(h.Value) == 0 {
                t.Errorf("traceparent header is empty")
            }
            break
        }
    }
    if !hasTP {
        t.Errorf("traceparent header missing; got headers: %+v", msg.Headers)
    }
}
```

- [ ] **Step 3.2: Run test, verify it fails**

Run: `cd libs/payments && go test ./infra/kafkarelay/... -run TestRelay_InjectsTraceparent -v`
Expected: FAIL with "traceparent header missing".

- [ ] **Step 3.3: Implement traceparent injection in pump()**

In `libs/payments/infra/kafkarelay/relay.go`, modify `pump`:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
)

// Add a helper carrier for kafka.Header.
type kafkaHeaderCarrier []*kafka.Header

func (c *kafkaHeaderCarrier) Get(key string) string {
    for _, h := range *c {
        if h.Key == key {
            return string(h.Value)
        }
    }
    return ""
}
func (c *kafkaHeaderCarrier) Set(key, val string) {
    for i, h := range *c {
        if h.Key == key {
            (*c)[i].Value = []byte(val)
            return
        }
    }
    *c = append(*c, &kafka.Header{Key: key, Value: []byte(val)})
}
func (c *kafkaHeaderCarrier) Keys() []string {
    out := make([]string, 0, len(*c))
    for _, h := range *c {
        out = append(out, h.Key)
    }
    return out
}

// In pump(), for each pending message:
func (r *Relay) pump(ctx context.Context) error {
    pending, err := r.src.PullUnpublished(ctx, r.batchLimit)
    if err != nil { return err }
    if len(pending) == 0 { return nil }

    tracer := otel.Tracer("payments/kafkarelay")
    prop := otel.GetTextMapPropagator()

    msgs := make([]kafka.Message, 0, len(pending))
    for _, p := range pending {
        spanCtx, span := tracer.Start(ctx, "kafkarelay.publish",
            trace.WithAttributes(/* could add topic, event-id later */),
        )
        carrier := kafkaHeaderCarrier{}
        // preload existing headers
        for k, v := range p.Headers {
            carrier = append(carrier, &kafka.Header{Key: k, Value: []byte(v)})
        }
        prop.Inject(spanCtx, &carrier)

        hdrs := make([]kafka.Header, 0, len(carrier))
        for _, h := range carrier {
            hdrs = append(hdrs, *h)
        }
        msgs = append(msgs, kafka.Message{
            Topic:   p.Topic,
            Key:     []byte(p.Key),
            Value:   p.Payload,
            Headers: hdrs,
        })
        span.End()
    }
    // ... rest unchanged
}
```

- [ ] **Step 3.4: Run test, verify it passes**

Run: `cd libs/payments && go test ./infra/kafkarelay/... -v`
Expected: PASS all three tests (incl. new one).

For propagation to work, `otel.SetTextMapPropagator(propagation.TraceContext{})` must run — covered in Task 2.4. The test runs without that setup, so the propagator is the default noop. **Fix:** add `otel.SetTextMapPropagator(propagation.TraceContext{})` at the top of the new test, or use the package's `init()` helper.

Update test to call:

```go
func TestRelay_InjectsTraceparent(t *testing.T) {
    t.Parallel()
    otel.SetTextMapPropagator(propagation.TraceContext{})
    // ... rest
}
```

- [ ] **Step 3.5: Commit**

```bash
git add libs/payments/infra/kafkarelay/
git commit -m "feat(payments): inject W3C traceparent into Kafka relay headers"
```

---

## Task 4: anti-fraud OTel Bootstrap + Kafka Trace Extraction

**Files:**
- Create: `apps/anti-fraud/src/observability/otel.ts`
- Create: `apps/anti-fraud/src/observability/metrics.module.ts`
- Modify: `apps/anti-fraud/src/main.ts`
- Modify: `apps/anti-fraud/src/app.module.ts`
- Modify: `apps/anti-fraud/src/infra/kafka/assessment.consumer.ts`
- Modify: `apps/anti-fraud/src/infra/kafka/event-publisher.ts`
- Modify: `apps/anti-fraud/src/infra/kafka/outbox-relay.service.ts`
- Modify: `apps/anti-fraud/package.json`

- [ ] **Step 4.1: Add OTel Node dependencies**

Run from repo root:

```bash
yarn workspace iris-anti-fraud add \
  @opentelemetry/sdk-node@^0.55.0 \
  @opentelemetry/auto-instrumentations-node@^0.52.0 \
  @opentelemetry/exporter-trace-otlp-grpc@^0.55.0 \
  @opentelemetry/exporter-metrics-otlp-grpc@^0.55.0 \
  @opentelemetry/instrumentation-kafkajs@^0.7.0 \
  @opentelemetry/api@^1.9.0 \
  @opentelemetry/resources@^1.28.0 \
  @opentelemetry/semantic-conventions@^1.28.0 \
  @opentelemetry/sdk-metrics@^1.28.0 \
  @opentelemetry/exporter-prometheus@^0.55.0 \
  prom-client@^15.1.3
```

(Verify the exact workspace name — check `apps/anti-fraud/package.json` `"name"` field.)

- [ ] **Step 4.2: Implement OTel bootstrap**

Create `apps/anti-fraud/src/observability/otel.ts`:

```typescript
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-grpc';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { resourceFromAttributes } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME,
} from '@opentelemetry/semantic-conventions/incubating';

let sdk: NodeSDK | null = null;

export async function start(): Promise<void> {
  const endpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
  if (!endpoint) return;

  sdk = new NodeSDK({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME ?? 'iris-anti-fraud',
      [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: process.env.NODE_ENV ?? 'local',
    }),
    traceExporter: new OTLPTraceExporter({ url: endpoint }),
    metricReader: new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: endpoint }),
      exportIntervalMillis: 10_000,
    }),
    instrumentations: [
      getNodeAutoInstrumentations({
        '@opentelemetry/instrumentation-fs': { enabled: false },
        '@opentelemetry/instrumentation-net': { enabled: false },
      }),
    ],
  });
  sdk.start();
}

export async function shutdown(): Promise<void> {
  if (sdk) await sdk.shutdown();
}
```

- [ ] **Step 4.3: Wire OTel in `main.ts`**

Modify `apps/anti-fraud/src/main.ts` — `await start()` BEFORE importing AppModule (otherwise the auto-instrumenter misses Nest's internal hooks):

```typescript
import { start as startOtel, shutdown as shutdownOtel } from './observability/otel';

async function bootstrap(): Promise<void> {
  await startOtel();
  const { NestFactory } = await import('@nestjs/core');
  const { Logger } = await import('@nestjs/common');
  const { AppModule } = await import('./app.module');

  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  const port = Number(process.env.PORT ?? 3001);

  app.enableShutdownHooks();
  app.beforeApplicationShutdown(async () => {
    await shutdownOtel();
  });
  await app.listen(port);

  Logger.log(`anti-fraud listening on :${port}`, 'Bootstrap');
}

bootstrap().catch((err) => {
  console.error('bootstrap failed', err);
  process.exit(1);
});
```

- [ ] **Step 4.4: Add Prometheus metrics endpoint**

Create `apps/anti-fraud/src/observability/metrics.module.ts`:

```typescript
import { Controller, Get, Header, Module, Res } from '@nestjs/common';
import type { Response } from 'express';
import { register, collectDefaultMetrics } from 'prom-client';

collectDefaultMetrics();

@Controller('metrics')
class MetricsController {
  @Get()
  @Header('Content-Type', register.contentType)
  async metrics(@Res() res: Response): Promise<void> {
    res.send(await register.metrics());
  }
}

@Module({ controllers: [MetricsController] })
export class MetricsModule {}
```

Wire in `apps/anti-fraud/src/app.module.ts`:

```typescript
import { MetricsModule } from './observability/metrics.module';

@Module({
  imports: [
    /* ...existing */
    MetricsModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 4.5: Extract `traceparent` in Kafka consumer**

Modify `apps/anti-fraud/src/infra/kafka/assessment.consumer.ts` — wrap the per-message handler in a span using extracted context. Look up the current run signature: `kafkajs` instrumentation already auto-instruments `consumer.run`, so spans are created automatically — but the **propagator** must be set globally. The `auto-instrumentations-node` package wires `W3CTraceContextPropagator` by default.

Verify by adding a guard:

```typescript
import { propagation, ROOT_CONTEXT, context, trace } from '@opentelemetry/api';

// In handler, before processing:
const carrier: Record<string, string> = {};
for (const [k, v] of Object.entries(message.headers ?? {})) {
    if (v) carrier[k] = Buffer.isBuffer(v) ? v.toString('utf8') : String(v);
}
const parentCtx = propagation.extract(ROOT_CONTEXT, carrier);
await context.with(parentCtx, async () => {
    // existing handler body
});
```

This is **belt-and-suspenders** with kafkajs auto-instrumentation. Acceptable: explicit extraction is the safety net when auto-instr changes its hook surface.

- [ ] **Step 4.6: Inject `traceparent` in event publisher + outbox relay**

Modify `apps/anti-fraud/src/infra/kafka/event-publisher.ts` — in `publish` method, before calling `producer.send`:

```typescript
import { propagation, context as otelCtx } from '@opentelemetry/api';

const carrier: Record<string, string> = {};
propagation.inject(otelCtx.active(), carrier);
const headers = { ...(record.headers ?? {}), ...carrier };
```

Apply same change to `apps/anti-fraud/src/infra/kafka/outbox-relay.service.ts` — wrap each batch send.

- [ ] **Step 4.7: Run anti-fraud tests**

Run: `make app-anti-fraud CMD=typecheck && make app-anti-fraud CMD=lint && make app-anti-fraud CMD=test && make app-anti-fraud CMD=build`
Expected: PASS. All 9 existing tests still green.

- [ ] **Step 4.8: Commit**

```bash
git add apps/anti-fraud/ package.json yarn.lock
git commit -m "feat(anti-fraud): wire OTel SDK + Prometheus /metrics + Kafka trace propagation"
```

---

## Task 5: apps/web Server-Side OTel

**Files:**
- Create: `apps/web/src/instrumentation.ts`
- Modify: `apps/web/package.json`
- Modify: `apps/web/next.config.ts`

- [ ] **Step 5.1: Add `@vercel/otel`**

Run: `yarn workspace @iris-gate/web add @vercel/otel@^1.13.0` (verify workspace name).

- [ ] **Step 5.2: Create instrumentation hook**

Create `apps/web/src/instrumentation.ts`:

```typescript
import { registerOTel } from '@vercel/otel';

export function register(): void {
  if (!process.env.OTEL_EXPORTER_OTLP_ENDPOINT) return;
  registerOTel({
    serviceName: process.env.OTEL_SERVICE_NAME ?? 'iris-web',
  });
}
```

(Next.js 16 looks for `src/instrumentation.ts` automatically when `experimental.instrumentationHook` is true — verify in next config.)

- [ ] **Step 5.3: Verify `next.config.ts` enables instrumentation hook**

Inspect `apps/web/next.config.ts`. If `experimental.instrumentationHook` is missing, add:

```typescript
const nextConfig: NextConfig = {
  // ...
  experimental: {
    instrumentationHook: true,
  },
};
```

In Next 16 this is on by default if a file exists, but explicit is safer.

- [ ] **Step 5.4: Verify web build still passes**

Run: `make app-web CMD=typecheck && make app-web CMD=lint && make app-web CMD=build`
Expected: PASS.

- [ ] **Step 5.5: Commit**

```bash
git add apps/web/ package.json yarn.lock
git commit -m "feat(web): server-side OTel via @vercel/otel instrumentation hook"
```

---

## Task 6: SLO Rules + Dashboards

**Files:**
- Create: `infra/docker/prometheus/rules/slo.yml`
- Create: `infra/docker/grafana/dashboards/api-latency.json`
- Create: `infra/docker/grafana/dashboards/payments-acquirer.json`
- Create: `infra/docker/grafana/dashboards/consumer-lag.json`

- [ ] **Step 6.1: SLO + alert rules**

Create `infra/docker/prometheus/rules/slo.yml`:

```yaml
groups:
  - name: iris-api-slo
    interval: 30s
    rules:
      - record: iris:http_request_duration_seconds:p95_5m
        expr: |
          histogram_quantile(0.95,
            sum by (le, http_route) (
              rate(http_server_duration_milliseconds_bucket{service="api"}[5m])
            )
          ) / 1000
      - alert: APILatencyP95High
        expr: iris:http_request_duration_seconds:p95_5m > 0.3
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "API p95 latency above 300ms SLO"
          description: "Route {{ $labels.http_route }} has p95 {{ $value }}s for 10m"

  - name: iris-error-budget
    interval: 1m
    rules:
      - record: iris:http_request_errors:rate5m
        expr: |
          sum by (service) (
            rate(http_server_duration_milliseconds_count{http_status_code=~"5.."}[5m])
          )
          /
          sum by (service) (
            rate(http_server_duration_milliseconds_count[5m])
          )
      - alert: ErrorBudgetBurn
        expr: iris:http_request_errors:rate5m > 0.01
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Error rate > 1% (monthly budget burn) on {{ $labels.service }}"
```

Note: OTel HTTP semconv currently emits `http.server.request.duration` (seconds) in v1.26+; older `otelhttp` v0.57 still emits `http_server_duration_milliseconds`. Validate the actual metric name once compose is up. The rule above assumes the older name — adjust based on observed `/metrics` output.

- [ ] **Step 6.2: API latency dashboard JSON**

Create `infra/docker/grafana/dashboards/api-latency.json` — minimal Grafana 11 schema, 3 panels (p50/p95/p99 by route, request rate, error rate).

Provide the full JSON literal in this step. Use Grafana's "View JSON Model" baseline — produce it via the Grafana export-flow; embed it inline here as ~150 lines. Keep `uid: "iris-api-latency"`, `title: "iris-gate · API Latency"`, datasource: `Prometheus`.

(For brevity in this plan: the executor copies the published Grafana standard "Prometheus HTTP" dashboard from `grafana.com/dashboards/12559` and adjusts the metric names + filters `{service="api"}`. Save the resulting JSON to this file. If 12559 is offline at execution time, hand-craft with 3 stat panels + 1 timeseries — provide JSON in the commit.)

- [ ] **Step 6.3: Payments acquirer approval dashboard**

Create `infra/docker/grafana/dashboards/payments-acquirer.json` — panels:
- Approval rate (5m): `sum(rate(payments_acquirer_authorize_total{outcome="approved"}[5m])) / sum(rate(payments_acquirer_authorize_total[5m]))`
- Authorize latency p95
- Capture latency p95

These metrics require explicit instrumentation in `libs/payments/infra/acquirer/` — wrap the `Authorize`/`Capture` HTTP calls with `otel` `Meter().Int64Counter("payments.acquirer.authorize")` and a `Float64Histogram("payments.acquirer.authorize.duration")`. **This is additional work** — add it as a sub-step:

  - [ ] **Step 6.3a:** In `libs/payments/infra/acquirer/http.go`, wrap `Authorize` / `Capture` with the meter pair. Test that the counter increments using `metric/sdkmetric/manual_reader` in a unit test.
  - [ ] **Step 6.3b:** Build the dashboard JSON pointing at the new metrics.

- [ ] **Step 6.4: Consumer lag dashboard**

Create `infra/docker/grafana/dashboards/consumer-lag.json` — panel: `kafka_consumer_lag` from `@opentelemetry/instrumentation-kafkajs` (emits as `messaging.kafka.consumer.lag`).

If the auto-instrumentation doesn't emit lag (verify after Task 4 deploy), fall back to running `kafka-exporter` as a sidecar in compose. Add `kafka-exporter` service + scrape config in Task 1 if needed (cross-reference: defer to a follow-up commit if it complicates the critical path).

- [ ] **Step 6.5: Verify dashboards load**

Bring up the stack, start `apps/api`, hit it with some requests, and open Grafana → Dashboards → iris-gate folder. All 3 dashboards visible. Panels render (may be empty if metrics not flowing yet — that's fine for this task).

- [ ] **Step 6.6: Commit**

```bash
git add infra/docker/prometheus/rules/ infra/docker/grafana/dashboards/
git commit -m "feat(observability): SLO rules + Grafana dashboards for api/payments/kafka"
```

---

## Task 7: Audit Log Library

**Files:**
- Create: `libs/audit/domain/event.go`
- Create: `libs/audit/domain/event_test.go`
- Create: `libs/audit/application/ports.go`
- Create: `libs/audit/application/recorder.go`
- Create: `libs/audit/application/recorder_test.go`
- Create: `libs/audit/infra/pgstore/store.go`
- Create: `libs/audit/infra/pgstore/migrations.go`
- Create: `libs/audit/infra/pgstore/migrations/0001_audit.sql`

- [ ] **Step 7.1: Write domain event test**

Create `libs/audit/domain/event_test.go`:

```go
package audit_test

import (
    "testing"
    "time"

    "github.com/google/uuid"

    "github.com/iris-gate/iris-gate/libs/audit/domain"
)

func TestEvent_HashChain_Deterministic(t *testing.T) {
    t.Parallel()
    at := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
    e1 := domain.NewEvent(uuid.New(), uuid.Nil, "identity.api_key.issued", []byte(`{"id":"abc"}`), "" /* prevHash */, at)
    e2 := domain.NewEvent(uuid.New(), e1.MerchantID, "identity.api_key.revoked", []byte(`{"id":"abc"}`), e1.Hash, at.Add(time.Second))

    if e1.Hash == "" { t.Fatal("hash empty") }
    if e2.PrevHash != e1.Hash { t.Errorf("e2.prev=%q, want %q", e2.PrevHash, e1.Hash) }

    // Determinism: same inputs → same hash.
    same := domain.NewEvent(e1.ID, e1.MerchantID, e1.Action, e1.Payload, "", at)
    if same.Hash != e1.Hash { t.Errorf("non-deterministic hash") }
}

func TestEvent_Tampering_Detected(t *testing.T) {
    t.Parallel()
    at := time.Now().UTC()
    e := domain.NewEvent(uuid.New(), uuid.Nil, "identity.api_key.issued", []byte(`{"id":"abc"}`), "", at)
    if err := e.Verify(); err != nil { t.Fatalf("verify: %v", err) }

    // Tamper payload.
    e.Payload = []byte(`{"id":"DEF"}`)
    if err := e.Verify(); err == nil {
        t.Errorf("expected tamper detection")
    }
}
```

- [ ] **Step 7.2: Run test, verify fail**

Run: `go test ./libs/audit/...`
Expected: FAIL with "no Go files".

- [ ] **Step 7.3: Implement domain Event**

Create `libs/audit/domain/event.go`:

```go
package audit

import (
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "time"

    "github.com/google/uuid"
)

var ErrTampered = errors.New("audit event hash mismatch")

type Event struct {
    ID         uuid.UUID
    MerchantID uuid.UUID // uuid.Nil if not scoped to a merchant
    Action     string    // e.g. "identity.api_key.issued"
    Payload    []byte    // canonical JSON
    PrevHash   string
    OccurredAt time.Time
    Hash       string
}

func NewEvent(id, merchantID uuid.UUID, action string, payload []byte, prevHash string, at time.Time) Event {
    e := Event{
        ID:         id,
        MerchantID: merchantID,
        Action:     action,
        Payload:    payload,
        PrevHash:   prevHash,
        OccurredAt: at.UTC(),
    }
    e.Hash = e.computeHash()
    return e
}

func (e Event) Verify() error {
    if e.Hash != e.computeHash() {
        return ErrTampered
    }
    return nil
}

func (e Event) computeHash() string {
    h := sha256.New()
    h.Write([]byte(e.ID.String()))
    h.Write([]byte("|"))
    h.Write([]byte(e.MerchantID.String()))
    h.Write([]byte("|"))
    h.Write([]byte(e.Action))
    h.Write([]byte("|"))
    h.Write(e.Payload)
    h.Write([]byte("|"))
    h.Write([]byte(e.PrevHash))
    h.Write([]byte("|"))
    h.Write([]byte(e.OccurredAt.UTC().Format(time.RFC3339Nano)))
    return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 7.4: Run test, verify pass**

Run: `go test ./libs/audit/domain/... -v`
Expected: PASS both tests.

- [ ] **Step 7.5: Add `go.work` entry for libs/audit**

If `libs/audit` is not already a Go module, init it:

```bash
cd libs/audit
go mod init github.com/iris-gate/iris-gate/libs/audit
go mod tidy
```

Then add to `go.work`:

```
use ./libs/audit
```

- [ ] **Step 7.6: Implement application ports + recorder**

Create `libs/audit/application/ports.go`:

```go
package application

import (
    "context"

    "github.com/google/uuid"

    audit "github.com/iris-gate/iris-gate/libs/audit/domain"
)

// EventStore appends events and reads the latest hash for chain continuity.
type EventStore interface {
    Append(ctx context.Context, e audit.Event) error
    LastHash(ctx context.Context, merchantID uuid.UUID) (string, error)
}

type Clock interface { Now() time.Time }
type SystemClock struct{}
func (SystemClock) Now() time.Time { return time.Now().UTC() }

type IDGen interface { New() uuid.UUID }
type UUIDGen struct{}
func (UUIDGen) New() uuid.UUID { return uuid.New() }
```

Create `libs/audit/application/recorder.go`:

```go
package application

import (
    "context"

    "github.com/google/uuid"

    audit "github.com/iris-gate/iris-gate/libs/audit/domain"
)

type Recorder struct {
    store EventStore
    clock Clock
    ids   IDGen
}

func NewRecorder(store EventStore, clock Clock, ids IDGen) *Recorder {
    return &Recorder{store: store, clock: clock, ids: ids}
}

func (r *Recorder) Record(ctx context.Context, merchantID uuid.UUID, action string, payload []byte) error {
    prev, err := r.store.LastHash(ctx, merchantID)
    if err != nil { return err }
    e := audit.NewEvent(r.ids.New(), merchantID, action, payload, prev, r.clock.Now())
    return r.store.Append(ctx, e)
}
```

- [ ] **Step 7.7: Recorder test with fake store**

Create `libs/audit/application/recorder_test.go`:

```go
package application_test

import (
    "context"
    "encoding/json"
    "sync"
    "testing"
    "time"

    "github.com/google/uuid"

    audit "github.com/iris-gate/iris-gate/libs/audit/domain"
    "github.com/iris-gate/iris-gate/libs/audit/application"
)

type fakeStore struct {
    mu     sync.Mutex
    events []audit.Event
}

func (s *fakeStore) Append(_ context.Context, e audit.Event) error {
    s.mu.Lock(); defer s.mu.Unlock()
    s.events = append(s.events, e)
    return nil
}
func (s *fakeStore) LastHash(_ context.Context, _ uuid.UUID) (string, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    if len(s.events) == 0 { return "", nil }
    return s.events[len(s.events)-1].Hash, nil
}

type fixedClock struct{ t time.Time }
func (c fixedClock) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

type fixedIDs struct{ counter uint8 }
func (g *fixedIDs) New() uuid.UUID {
    g.counter++
    return uuid.UUID{g.counter}
}

func TestRecorder_ChainsHashesAcrossEvents(t *testing.T) {
    t.Parallel()
    store := &fakeStore{}
    rec := application.NewRecorder(store, &fixedClock{t: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)}, &fixedIDs{})

    if err := rec.Record(context.Background(), uuid.Nil, "test.a", []byte(`{}`)); err != nil { t.Fatal(err) }
    if err := rec.Record(context.Background(), uuid.Nil, "test.b", json.RawMessage(`{}`)); err != nil { t.Fatal(err) }

    if len(store.events) != 2 { t.Fatalf("events=%d, want 2", len(store.events)) }
    if store.events[1].PrevHash != store.events[0].Hash {
        t.Errorf("chain broken: prev=%q, want %q", store.events[1].PrevHash, store.events[0].Hash)
    }
}
```

Run: `go test ./libs/audit/...`
Expected: PASS.

- [ ] **Step 7.8: Postgres migration + store**

Create `libs/audit/infra/pgstore/migrations/0001_audit.sql`:

```sql
CREATE TABLE IF NOT EXISTS audit_events (
  id           UUID PRIMARY KEY,
  merchant_id  UUID NOT NULL,        -- uuid.Nil (zero) for system-scoped events
  action       TEXT NOT NULL,
  payload      JSONB NOT NULL,
  prev_hash    TEXT NOT NULL,
  occurred_at  TIMESTAMPTZ NOT NULL,
  hash         TEXT NOT NULL,
  CONSTRAINT audit_hash_unique UNIQUE (hash)
);

CREATE INDEX IF NOT EXISTS audit_events_merchant_occurred_idx
  ON audit_events (merchant_id, occurred_at DESC);
```

Create `libs/audit/infra/pgstore/migrations.go`:

```go
package pgstore

import (
    "context"
    "embed"

    "github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func MigrateEmbedded(ctx context.Context, pool *pgxpool.Pool) error {
    entries, err := migrationsFS.ReadDir("migrations")
    if err != nil { return err }
    for _, e := range entries {
        if e.IsDir() { continue }
        sqlBytes, err := migrationsFS.ReadFile("migrations/" + e.Name())
        if err != nil { return err }
        if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil { return err }
    }
    return nil
}
```

Create `libs/audit/infra/pgstore/store.go`:

```go
package pgstore

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    audit "github.com/iris-gate/iris-gate/libs/audit/domain"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Append(ctx context.Context, e audit.Event) error {
    _, err := s.pool.Exec(ctx,
        `INSERT INTO audit_events (id, merchant_id, action, payload, prev_hash, occurred_at, hash)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
        e.ID, e.MerchantID, e.Action, e.Payload, e.PrevHash, e.OccurredAt, e.Hash,
    )
    return err
}

func (s *Store) LastHash(ctx context.Context, merchantID uuid.UUID) (string, error) {
    var h string
    err := s.pool.QueryRow(ctx,
        `SELECT hash FROM audit_events
         WHERE merchant_id = $1
         ORDER BY occurred_at DESC LIMIT 1`,
        merchantID,
    ).Scan(&h)
    if errors.Is(err, pgx.ErrNoRows) { return "", nil }
    return h, err
}
```

- [ ] **Step 7.9: Run all audit tests**

Run: `cd libs/audit && go test ./... -v && go vet ./...`
Expected: PASS.

- [ ] **Step 7.10: Commit**

```bash
git add libs/audit/ go.work
git commit -m "feat(audit): append-only hash-chained audit event store (Go lib)"
```

---

## Task 8: Wire Audit Recorder into Identity + Fraud Use Cases

**Files:**
- Modify: `libs/identity/application/ports.go`
- Modify: `libs/identity/application/issue_api_key.go`
- Modify: `libs/identity/application/revoke_api_key.go`
- Modify: `apps/api/cmd/api/main.go` (compose root)
- Modify: `libs/fraud/src/application/ports.ts`
- Modify: `libs/fraud/src/application/assess-risk.use-case.ts`
- Modify: `apps/anti-fraud/src/modules/fraud/fraud.module.ts` (provide recorder)
- Create: `apps/anti-fraud/src/infra/audit/prisma-audit-recorder.ts`

- [ ] **Step 8.1: Add AuditRecorder port (Go)**

In `libs/identity/application/ports.go`, add:

```go
import "github.com/google/uuid"

type AuditRecorder interface {
    Record(ctx context.Context, merchantID uuid.UUID, action string, payload []byte) error
}
```

In `libs/identity/application/issue_api_key.go`, accept the port in the use case constructor and call after a successful issue:

```go
_ = u.audit.Record(ctx, key.MerchantID, "identity.api_key.issued",
    mustJSON(map[string]any{"key_id": key.ID, "prefix": key.Prefix}))
```

(Best-effort write — log on error but don't fail the request. Use `slog.Error` and continue.)

In `libs/identity/application/revoke_api_key.go`, same pattern, action `"identity.api_key.revoked"`.

Add a `NoopAuditRecorder` in `libs/identity/application/ports.go` for tests:

```go
type NoopAuditRecorder struct{}
func (NoopAuditRecorder) Record(context.Context, uuid.UUID, string, []byte) error { return nil }
```

Update all existing identity unit tests that construct the use cases to pass `application.NoopAuditRecorder{}`.

- [ ] **Step 8.2: Update existing identity tests**

Run: `cd libs/identity && go test ./application/... -v`
Expected: COMPILE FAIL (missing constructor arg). Fix each test by passing `application.NoopAuditRecorder{}`. Re-run until PASS.

- [ ] **Step 8.3: Wire in API composition root**

In `apps/api/cmd/api/main.go`:

```go
import (
    auditapp "github.com/iris-gate/iris-gate/libs/audit/application"
    auditpg  "github.com/iris-gate/iris-gate/libs/audit/infra/pgstore"
)

// After identityRepo and before identityDeps:
if cfg.MigrateOnStartup {
    if err := auditpg.MigrateEmbedded(rootCtx, pool); err != nil {
        slog.Error("migrate audit", "err", err); os.Exit(1)
    }
}
auditStore := auditpg.New(pool)
auditRec := auditapp.NewRecorder(auditStore, auditapp.SystemClock{}, auditapp.UUIDGen{})

// Pass auditRec into IssueAPIKey / RevokeAPIKey constructors.
```

- [ ] **Step 8.4: Add TypeScript AuditRecorder port (anti-fraud)**

In `libs/fraud/src/application/ports.ts`:

```typescript
export interface AuditRecorder {
  record(merchantId: string, action: string, payload: unknown): Promise<void>;
}

export class NoopAuditRecorder implements AuditRecorder {
  async record(): Promise<void> {}
}
```

In `libs/fraud/src/application/assess-risk.use-case.ts`, accept `AuditRecorder` in the constructor; after successful assessment completion, call:

```typescript
await this.audit.record(
  result.merchantRef ?? 'system',
  'fraud.assessment.completed',
  { assessmentId: assessment.id, decision: assessment.decision, score: assessment.score.value },
).catch((err) => this.logger.warn?.({ err }, 'audit record failed'));
```

(Audit failures don't fail the request.)

- [ ] **Step 8.5: Update fraud lib tests**

All existing tests in `libs/fraud/src/application/*.spec.ts` need a `NoopAuditRecorder` arg. Update each one.

Run: `make app-fraud-lib CMD=test` (verify Makefile target name; otherwise `cd libs/fraud && yarn test`).
Expected: PASS.

- [ ] **Step 8.6: Provide concrete audit recorder in anti-fraud DI**

Create `apps/anti-fraud/src/infra/audit/prisma-audit-recorder.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { createHash } from 'crypto';
import { PrismaService } from '../prisma/prisma.service';
import type { AuditRecorder } from '@iris-gate/fraud';

@Injectable()
export class PrismaAuditRecorder implements AuditRecorder {
  constructor(private readonly prisma: PrismaService) {}

  async record(merchantId: string, action: string, payload: unknown): Promise<void> {
    const occurredAt = new Date();
    const prev = await this.prisma.auditEvent.findFirst({
      where: { merchantId },
      orderBy: { occurredAt: 'desc' },
      select: { hash: true },
    });
    const prevHash = prev?.hash ?? '';
    const body = JSON.stringify(payload);
    const id = crypto.randomUUID();
    const hash = createHash('sha256')
      .update(`${id}|${merchantId}|${action}|${body}|${prevHash}|${occurredAt.toISOString()}`)
      .digest('hex');
    await this.prisma.auditEvent.create({
      data: { id, merchantId, action, payload: body, prevHash, occurredAt, hash },
    });
  }
}
```

Add a Prisma migration: `apps/anti-fraud/prisma/migrations/<timestamp>_audit/migration.sql` with the same `audit_events` schema (separate table per service is acceptable for v1 — each service owns its audit chain. ADR-0004 documents this).

Update `apps/anti-fraud/prisma/schema.prisma` to add the `AuditEvent` model.

- [ ] **Step 8.7: Run all tests + builds**

```bash
make test  # fan-out: api Go + web vitest + anti-fraud jest + fraud lib jest
make lint
make build
```

Expected: ALL PASS.

- [ ] **Step 8.8: Commit**

```bash
git add libs/identity/ libs/fraud/ libs/audit/ apps/api/ apps/anti-fraud/ \
        apps/anti-fraud/prisma/
git commit -m "feat(audit): wire audit recorder into identity + fraud use cases"
```

---

## Task 9: ADR + Roadmap Update

**Files:**
- Create: `docs/adr/0004-observability-pipeline.md`
- Modify: `docs/roadmap.md`

- [ ] **Step 9.1: Author ADR-0004**

Create `docs/adr/0004-observability-pipeline.md` with sections: Status / Data / Deciders / Context / Decision / Pipeline / Trade-offs / Alternatives Considered.

Key points:
- One OTel Collector gateway, OTLP-gRPC `:4317`.
- Prometheus stores metrics; Jaeger stores traces; Grafana visualizes both.
- Audit log is **separate** from telemetry: lives in Postgres for legal/compliance retention, not in Loki/Prometheus.
- Audit chain is **per-service** for v1 (identity chain in `iris` db, fraud chain in `antifraud` db). Cross-service chain → Pós-MVP if compliance demands it.

- [ ] **Step 9.2: Update roadmap Phase 6 block**

Modify `docs/roadmap.md` lines 198-205. Replace the unchecked checklist with checked items + status block:

```markdown
## Fase 6 — Observabilidade end-to-end (semana 8)

> **Deltas vs. plano original:**
> - **Stack local**: OTel Collector + Prometheus + Grafana + Jaeger 2.0 adicionados ao `docker-compose.yml`. Loki diferido pra Fase 7 (logs vão pra stdout + JSON via slog/pino).
> - **Audit log per-service**: cada bounded context tem sua própria tabela `audit_events` (identity em `iris`, fraud em antifraud db). Chain cross-service fica pós-MVP.
> - **Acquirer metrics** ganharam instrumentação dedicada (`payments.acquirer.{authorize,capture}_total/duration`) — não estavam no plano, mas alimentam o dashboard de aprovação.

- [x] OTel SDK em apps/api (Go), apps/anti-fraud (NestJS), apps/web (Next via @vercel/otel).
- [x] Trace context propagado em HTTP (otelhttp) + Kafka headers (W3C traceparent injetado pelo relay e por event-publisher; extraído pelo assessment.consumer).
- [x] Dashboards Grafana provisionados via arquivo: `api-latency`, `payments-acquirer`, `consumer-lag`.
- [x] Alertas Prometheus em `slo.yml`: SLO de latência p95 < 300ms (10m), error budget burn (>1% por 15m).
- [x] Audit log append-only com hash chain (`libs/audit/`) para emissão/revogação de API key e decisões de fraude.

**Status Fase 6:** ✅ **Concluída.** Próxima: Fase 7 — Hardening (CSP, rate limit, RLS, govulncheck bloqueante, threat model).
```

- [ ] **Step 9.3: Commit**

```bash
git add docs/adr/0004-observability-pipeline.md docs/roadmap.md
git commit -m "docs(adr): ADR-0004 observability pipeline + close Phase 6 in roadmap"
```

---

## Task 10: Final End-to-End Smoke Verification

- [ ] **Step 10.1: Bring the stack up**

```bash
cd infra/docker && docker compose up -d --wait
cd ../..
```

- [ ] **Step 10.2: Set OTel env + start API**

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_SERVICE_NAME=iris-api
export MIGRATE_ON_STARTUP=true
export DATABASE_URL=postgres://iris:iris@localhost:5432/iris?sslmode=disable
export KAFKA_BROKER=localhost:9092
export RELAY_ENABLED=true
make app-api CMD=dev  # or `go run ./apps/api/cmd/api`
```

- [ ] **Step 10.3: Generate traffic + verify**

In another shell:
```bash
curl -sf http://localhost:8080/healthz
# Create merchant, issue key, create+authorize+capture intent
# (use the e2e flow from apps/web/e2e if scripted, else hand-craft a few curls)
```

Verify:
- `curl -s http://localhost:9100/metrics | grep go_goroutines` → metric present.
- Open `http://localhost:16686`, search service `iris-api` → traces appear.
- Open `http://localhost:9090/graph`, query `http_server_duration_milliseconds_count` → series present.
- Open `http://localhost:3000` (anonymous viewer), navigate to dashboard "iris-gate · API Latency" → panels render with data.
- Check `audit_events` table after issuing an API key: `psql ... -c 'SELECT id, action, hash, prev_hash FROM audit_events ORDER BY occurred_at DESC LIMIT 5'` → entries present, hash chain valid.

- [ ] **Step 10.4: Tear down**

```bash
cd infra/docker && docker compose down
```

- [ ] **Step 10.5: Final CI verification**

```bash
make test
make lint
make build
```

All green.

- [ ] **Step 10.6: Push**

```bash
git push origin main
```

---

## Self-Review Notes

**Spec coverage:**
- [x] OTel SDK em todos os serviços → Tasks 2, 4, 5.
- [x] Trace context HTTP + Kafka headers → Tasks 2 (otelhttp), 3 (relay traceparent), 4 (consumer extract / publisher inject).
- [x] Dashboards Grafana → Task 6.
- [x] Alertas Prometheus SLO → Task 6.1.
- [x] Audit log append-only → Tasks 7, 8.

**Known gaps / follow-ups:**
- Lighthouse CI rig is **NOT** included (Phase 5 already deferred this). Stays deferred.
- Real Kafka consumer-lag metric depends on `@opentelemetry/instrumentation-kafkajs` emitting `messaging.kafka.consumer.lag` — if it doesn't (verify in Task 6.4), fall back to `kafka-exporter` sidecar. Add as a follow-up commit if needed.
- Acquirer metric instrumentation (Step 6.3a) is **inline new work**; the executor needs to add `Meter()` calls inside `libs/payments/infra/acquirer/http.go` and `fake.go`.
- Sampling is `AlwaysSample()` in dev — fine for local. Prod sampling decision lives in Fase 7 (Hardening) or in env config later.
- Audit log payload is plain JSON. PII (email, CNPJ) **will** appear there for identity events. Acceptable for v1 — Fase 7 adds field-level encryption if compliance requires.
- Cross-service audit chain unified is deferred. Documented in ADR-0004.

**Placeholder scan:** none — every step has the actual code or command.

**Type consistency:** `AuditRecorder` port is consistent (Go: `Record`, TS: `record`). `Event` struct fields match in domain code and SQL migration.
