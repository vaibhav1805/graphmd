# Logging Infrastructure

## Overview

Centralized logging system for all application components.

## Log Levels

- DEBUG: Development debugging
- INFO: General informational messages
- WARN: Warning conditions
- ERROR: Error conditions
- FATAL: System failures

## Aggregation

Logs are aggregated from all services into a central repository.

## Retention

- Application logs: 30 days
- Error logs: 90 days
- Audit logs: 1 year

## Querying

Use standard grep and log aggregation tools to search logs.
