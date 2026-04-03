# SMS Service

## Overview

SMS delivery and carrier integration. Handles SMS sending, delivery tracking, and two-way messaging capabilities. Integrates with Twilio or AWS SNS.

## Responsibilities

- SMS sending and delivery
- Delivery status tracking (sent, delivered, failed)
- Inbound SMS handling
- Shortcode management
- DLR (Delivery Receipt) handling

## Key Dependencies

- **notification-service**: Receives SMS send requests
- **database-service**: Delivery logs and message history
- **logging-service**: SMS delivery audit trail
- **monitoring-service**: Queue depth and success rate tracking

## SMS Provider Integration

Integrates with Twilio API. Webhooks from Twilio notify of delivery status changes.

## Compliance

- TCPA compliance (consent tracking)
- Carrier filtering (blocks unwanted keywords)
- Two-way messaging support for customer service

## Delivery Guarantees

- Retry up to 3 times on carrier failure
- Dead letter queue for permanent failures
- DLR callback processing within 100ms

## Performance

- Queue depth: <5000 messages
- P99 delivery: <2 seconds to carrier
