# Email Service

## Overview

Email delivery provider integration. Handles SMTP communication, bounce tracking, unsubscribe management, and delivery metrics. Integrates with SendGrid or AWS SES.

## Responsibilities

- Email sending via SMTP or provider API
- Bounce and complaint tracking
- Unsubscribe list management
- Email templates and variable substitution
- Delivery metrics and reporting

## Key Dependencies

- **notification-service**: Receives email send requests
- **database-service**: Bounce tracking and preference storage
- **logging-service**: Email delivery logs and error tracking
- **monitoring-service**: Email queue depth and latency metrics

## Email Providers

Primary: SendGrid with fallback to AWS SES. Provider credentials managed via environment variables.

## Bounce Handling

Hard bounces (invalid email) are stored in **database-service** and the user is marked as unsubscribed. Soft bounces trigger retry logic.

## Compliance

- Unsubscribe links mandatory in all emails
- List-Unsubscribe header on all transactional emails
- CAN-SPAM Act compliance

## Performance

- Queue depth: <1000 emails
- P99 delivery latency: <10 seconds
- Success rate: >99.5%
