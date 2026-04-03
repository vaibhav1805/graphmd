# API Gateway Service

## Overview

Central request router and load balancer for all microservices. Routes incoming API requests to appropriate backend services, handles authentication validation via the auth-service, applies rate limiting, and manages request/response transformation.

## Responsibilities

- Request routing based on URL patterns
- API version management (v1, v2)
- Rate limiting and throttling
- Request logging and monitoring
- Response aggregation
- Cross-Origin Resource Sharing (CORS) handling

## Key Dependencies

The API Gateway routes requests to:
- **auth-service**: Validates JWT tokens for secured endpoints
- **user-service**: User profile and account management
- **order-service**: Order operations
- **payment-service**: Payment processing
- **search-service**: Full-text search across orders and products
- **notification-service**: Notification endpoints

All requests are logged to the **logging-service** for audit trails and debugging.

## Configuration

API routing rules are defined in `routes.yaml`. Each backend service is registered with health check endpoints and timeout configurations.

## Performance

- P99 latency: <100ms for simple routing
- Handles 10k RPS with response aggregation from multiple services
- Graceful degradation when backend services are slow (timeout at 30s)

## API Documentation

See the **documentation-service** for complete OpenAPI/Swagger specs.
