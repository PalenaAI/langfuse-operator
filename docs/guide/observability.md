# Observability

## Prometheus ServiceMonitor

Enable a Prometheus ServiceMonitor to scrape operator and Langfuse metrics:

```yaml
spec:
  observability:
    serviceMonitor:
      enabled: true
      interval: "30s"
      labels:
        release: prometheus       # match your Prometheus selector
```

### Operator Metrics

| Metric | Type | Description |
|---|---|---|
| `langfuse_operator_reconcile_total` | Counter | Reconciliation count by controller and result |
| `langfuse_operator_reconcile_errors_total` | Counter | Error count by controller |
| `langfuse_operator_reconcile_duration_seconds` | Histogram | Reconciliation duration |
| `langfuse_operator_managed_instances` | Gauge | Number of managed instances |

### Instance Metrics

| Metric | Type | Description |
|---|---|---|
| `langfuse_instance_web_replicas` | Gauge | Web replica count |
| `langfuse_instance_worker_replicas` | Gauge | Worker replica count |
| `langfuse_instance_worker_queue_depth` | Gauge | Worker queue depth |
| `langfuse_instance_clickhouse_storage_used_bytes` | Gauge | ClickHouse disk usage |
| `langfuse_instance_circuit_breaker_active` | Gauge | Circuit breaker state (0/1) |
| `langfuse_instance_secret_rotation_total` | Counter | Secret rotation events |

## OpenTelemetry

Send traces from Langfuse itself to an OTEL collector:

```yaml
spec:
  observability:
    otel:
      enabled: true
      endpoint: "otel-collector.monitoring.svc:4317"
      protocol: grpc       # grpc | http
```

## Langfuse Telemetry

Langfuse sends anonymous usage telemetry by default. To disable:

```yaml
spec:
  security:
    telemetry:
      enabled: false
```
