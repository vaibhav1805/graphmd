# Search Service

## Overview

Full-text search engine for orders, products, and users. Provides fast, faceted search across the platform using Elasticsearch. Enables complex queries with relevance ranking.

## Responsibilities

- Index orders, products, and users in Elasticsearch
- Full-text search with relevance scoring
- Faceted search (filters by category, price, date range)
- Autocomplete/suggestions
- Search analytics

## Key Dependencies

- **order-service**: Sends order events for indexing
- **inventory-service**: Product data for indexing
- **user-service**: User profile indexing
- **database-service**: Reads historical data for initial indexing
- **logging-service**: Search query logs and performance metrics
- **monitoring-service**: Search latency and QPS tracking

## Indexing Strategy

Events are indexed asynchronously:
- Order created/updated → index in Elasticsearch within 5 seconds
- Product stock changes → index within 30 seconds (lower priority)
- User profile changes → index within 1 minute

## Search Capabilities

- Search across multiple indexes simultaneously
- Relevance boosting for recent orders
- Filter by date range, customer, status
- Aggregations (e.g., "orders by date", "revenue by category")

## Performance

- Search latency: <500ms for typical queries
- Index size: ~2GB per 1M documents
- Replication across 3 nodes for high availability
