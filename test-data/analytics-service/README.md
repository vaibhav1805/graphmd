# Analytics Service

## Overview

Business intelligence and analytics platform. Processes event streams from all services to generate business metrics, dashboards, and reports.

## Responsibilities

- Event stream processing (Kafka/Kinesis)
- Data warehouse ETL (extract, transform, load)
- Metric calculation and aggregation
- Business dashboard creation
- Custom report generation

## Key Dependencies

- **logging-service**: Event source for analytics processing
- **database-service**: Warehouse storage and historical data
- **order-service**: Order events and funnel analysis
- **user-service**: User acquisition and cohort analysis
- **payment-service**: Revenue reporting

## Analytics Dashboards

- Revenue by day/week/month
- Customer acquisition cost (CAC)
- Customer lifetime value (LTV)
- Order funnel (browse → cart → checkout → payment)
- User cohort retention
- Inventory turnover
- Payment success rates

## Data Warehouse

- Snowflake/BigQuery for OLAP queries
- Daily snapshot tables (users, orders, revenue)
- Fact tables: orders, payments, page views
- Dimension tables: users, products, categories

## Event Processing

Events are streamed from services:
- Order created → recorded in warehouse
- Payment completed → revenue added to dashboard
- User signup → cohort tracking started

## Performance

- Event latency: 5-15 minutes to dashboard (batch processing)
- Dashboard query: <5 seconds
- Historical data retention: 3 years
