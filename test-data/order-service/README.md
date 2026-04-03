# Order Service

## Overview

Order management and fulfillment service. Handles order creation, tracking, status updates, and order history. Core business logic for the e-commerce platform.

## Responsibilities

- Order creation and validation
- Order status tracking (pending, confirmed, shipped, delivered)
- Order history and retrieval
- Order cancellation and refunds coordination
- Return management

## Key Dependencies

- **user-service**: Customer lookup and validation
- **payment-service**: Payment authorization and capture
- **inventory-service**: Stock checking and reservations
- **notification-service**: Order confirmations, shipping updates, delivery notifications
- **search-service**: Full-text search for orders by ID, customer, product
- **database-service**: Order persistence and historical data
- **cache-service**: Recent orders and in-flight order caching
- **logging-service**: Order audit trail

## Order Processing Flow

1. User submits order via **api-gateway-service**
2. Validate customer via **user-service**
3. Check inventory via **inventory-service**
4. Process payment via **payment-service**
5. Create order record in **database-service**
6. Send confirmation via **notification-service**
7. Trigger fulfillment workflow

## Performance

- Order creation: <500ms
- Order lookup: <100ms (cached)
- Search: <1s for date range queries (via **search-service**)

## Integration with Fulfillment

Orders trigger fulfillment via **deployment-service** webhooks to warehouse systems.
