# Analytics and Tracking

## Overview

Collects and processes user behavior analytics.

## Events Tracked

- User login/logout
- Page views
- Button clicks
- Search queries
- Purchase events

## Data Processing

Events are collected in a message queue and processed asynchronously.

## Retention Policy

- Raw events: 90 days
- Aggregated metrics: 2 years
- User identifiers: 30 days post-deletion

## Privacy

- PII is encrypted at rest
- Access requires authentication
- Audit logs track all queries
