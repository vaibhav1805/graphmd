# Logging Service

## Overview

Centralized logging and log aggregation platform. Collects logs from all services, provides indexing, searching, and real-time alerting. Integrates with ELK stack (Elasticsearch, Logstash, Kibana).

## Responsibilities

- Log ingestion from all services (via syslog, HTTP, or agents)
- Log parsing and enrichment
- Full-text search and analytics
- Real-time alerting on error patterns
- Log retention and archival

## Key Dependencies

- **monitoring-service**: Alert execution and Slack notifications
- All other services: Send structured logs

## Log Consumers

All services log to logging-service:
- **auth-service**: Authentication attempts, token issuance
- **order-service**: Order events and status changes
- **payment-service**: Financial transactions (compliance required)
- **api-gateway-service**: Request/response logs
- **email-service**: Email delivery logs
- **sms-service**: SMS delivery logs

## Log Format

All logs are JSON:
```json
{
  "timestamp": "2026-03-08T12:34:56Z",
  "service": "order-service",
  "level": "info",
  "message": "Order created",
  "order_id": "ORD-123456",
  "user_id": "USR-789",
  "trace_id": "abc123def456"
}
```

## Storage

- Hot storage: Elasticsearch (7 days)
- Warm storage: S3 (30 days, monthly aggregates)
- Cold storage: Glacier (1 year for audits)

## Performance

- Ingest rate: 10k logs/sec
- Search latency: <500ms for date range queries
- Real-time alerting: <10 second detection lag
