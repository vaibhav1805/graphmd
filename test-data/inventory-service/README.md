# Inventory Service

## Overview

Stock and inventory management. Tracks product availability, handles stock reservations for orders, updates stock levels, and alerts when inventory is low.

## Responsibilities

- Stock level tracking
- Stock reservation for pending orders
- Stock depletion on order fulfillment
- Low inventory alerts
- Restock notifications to operations team
- Product catalog management

## Key Dependencies

- **database-service**: Persistent stock records and history
- **cache-service**: Real-time stock cache (updated on every inventory change; TTL: 30 seconds for freshness)
- **order-service**: Receives stock reservation requests
- **notification-service**: Sends low stock alerts to operations team
- **search-service**: Product search integration (indexed by inventory)
- **logging-service**: Audit trail of stock changes

## Stock Reservation Model

When an order is placed:
1. Reserve stock in cache and database
2. Hold reservation for 30 minutes
3. If payment succeeds, convert to permanent depletion
4. If payment fails, release reservation

## Performance

- Stock lookup: <50ms (cached)
- Stock update: <200ms (cache + database write)
- Low stock alerts generated within 1 minute of threshold breach

## Integration with Supply Chain

Integrates with warehouse management system via **deployment-service** webhooks for restock orders.
