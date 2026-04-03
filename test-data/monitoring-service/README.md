# Monitoring Service

## Overview

System and application monitoring using Prometheus and Grafana. Collects metrics from all services, provides visualization dashboards, alerting, and incident management.

## Responsibilities

- Metric collection (via Prometheus exporters)
- Dashboard creation and visualization (Grafana)
- Alert rule definition and management
- On-call scheduling and incident escalation
- SLA tracking and reporting

## Key Dependencies

- **logging-service**: Alert notifications via Slack
- All other services: Prometheus `/metrics` endpoints

## Key Metrics Tracked

- **API Gateway**: Request latency, error rate, QPS
- **Auth Service**: Token validation latency, failed auth attempts
- **Order Service**: Order creation rate, status distribution
- **Payment Service**: Transaction success rate, reconciliation lag
- **Database Service**: Connection pool usage, query latency
- **Cache Service**: Hit rate, eviction rate, size
- **Search Service**: Index size, query latency

## Alert Rules

Critical alerts:
- Service error rate >5% for 5 minutes
- Payment reconciliation >1 hour behind
- Database connection pool >80% utilization
- Cache hit rate <70% (indicates cache thrashing)

## Alerting Integration

- PagerDuty for critical alerts
- Slack for warnings
- Email for informational

## Performance

- Metric ingestion: 100k metrics/minute
- Query latency: <1 second for dashboard render
- Retention: 15 days of raw metrics, 1 year of hourly rollups
