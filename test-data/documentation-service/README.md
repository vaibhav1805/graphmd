# Documentation Service

## Overview

API documentation and developer portal. Maintains OpenAPI/Swagger specs for all services, generates documentation from specs, and provides interactive API testing via Swagger UI.

## Responsibilities

- OpenAPI spec aggregation and versioning
- Interactive API documentation (Swagger UI)
- Code generation (SDKs for various languages)
- API changelog management
- Developer onboarding guides

## Key Dependencies

- **api-gateway-service**: Serves /api/docs endpoint
- **search-service**: Full-text search of API documentation
- **notification-service**: Sends API change notifications to subscribers

## Spec Management

Each service maintains its own OpenAPI spec in `openapi.yaml`:
- Auth Service: `/auth/v1/openapi.yaml`
- Order Service: `/orders/v1/openapi.yaml`
- User Service: `/users/v1/openapi.yaml`

Specs are versioned and stored in git. Breaking changes require version bump (v1 → v2).

## Documentation Endpoints

- `/api/docs` — Interactive Swagger UI for all endpoints
- `/api/docs/download` — OpenAPI spec download
- `/api/changelog` — API changes by version

## Developer Experience

- Live API testing via Swagger UI
- Request/response examples
- Error code documentation
- Rate limit headers documentation

## Integration

Auto-generate from specs:
- Python SDK (via OpenAPI Generator)
- JavaScript/Node SDK
- Go SDK
- Documentation site
