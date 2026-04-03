# Cache Service

## Overview

Distributed caching layer using Redis. Provides sub-millisecond data access for frequently needed information. Manages cache invalidation, TTL policies, and distributed locks for atomic operations.

## Responsibilities

- Key-value caching with TTL
- Cache invalidation on updates
- Distributed locks for idempotent operations
- Cache warming strategies
- Eviction policies (LRU)

## Key Dependencies

- **logging-service**: Cache hit/miss metrics
- **monitoring-service**: Cache size and eviction tracking

## Consumers

All services use the cache-service:
- **auth-service**: Session tokens (1-hour TTL)
- **user-service**: Active user profiles (5-minute TTL)
- **inventory-service**: Stock levels (30-second TTL)
- **payment-service**: Payment locks and recent transactions
- **notification-service**: Templates

## Cache Patterns

- **Write-through**: Updates cache and database synchronously
- **Cache-aside**: Services check cache; miss triggers database fetch
- **Distributed locks**: Used for idempotent payments, stock reservations

## Cluster Configuration

- 3-node Redis cluster
- Master-slave replication
- Sentinel for automatic failover
- RDB snapshots every 1 minute

## Performance

- Get latency: <1ms
- Set latency: <5ms
- Cluster throughput: >100k ops/sec
