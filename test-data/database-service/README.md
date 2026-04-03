# Database Service

## Overview

Centralized database abstraction layer. Manages PostgreSQL connections, connection pooling, query optimization, and database schema management. All services store persistent data through this service.

## Responsibilities

- PostgreSQL connection management and pooling
- Query execution with prepared statements
- Migration management (schema versions)
- Backup and disaster recovery
- Data access logging (audit trail)
- Query performance monitoring

## Key Dependencies

- **logging-service**: Query logs and slow query tracking
- **monitoring-service**: Connection pool metrics, query latency

## Consumers

All services depend on database-service:
- **user-service**: User profiles
- **auth-service**: Credentials and sessions
- **order-service**: Order records
- **payment-service**: Transaction logs
- **inventory-service**: Stock levels
- **search-service**: Historical data for indexing
- **notification-service**: Notification history
- **security-service**: Audit logs

## Connection Pooling

- Min connections: 10
- Max connections: 100
- Idle timeout: 5 minutes
- Connection wait timeout: 30 seconds

## Schema Management

Migrations are version-controlled in `schema/migrations/`. Each service owns its tables; cross-service queries go through service APIs, not direct table access.

## Performance

- Query P99: <100ms (with proper indexing)
- Connection acquisition: <10ms
- Backup window: Nightly at 2 AM UTC
