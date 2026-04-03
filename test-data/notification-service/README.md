# Notification Service

## Overview

Centralized notification platform. Handles sending emails, SMS, and push notifications to users. Manages notification templates, delivery tracking, and retry logic.

## Responsibilities

- Email notifications (welcome, confirmations, receipts)
- SMS notifications (order updates, alerts)
- Push notifications (mobile apps)
- Notification template management
- Delivery status tracking
- Unsubscribe management

## Key Dependencies

- **email-service**: Handles email delivery and bounce tracking
- **sms-service**: Handles SMS delivery and carrier routing
- **database-service**: Notification history and preferences
- **cache-service**: Notification template cache
- **user-service**: User contact information and preferences
- **logging-service**: Delivery audit trail and error logs

## Notification Flows

Called by:
- **order-service**: Order confirmation, tracking, delivery notifications
- **inventory-service**: Low stock alerts
- **payment-service**: Payment receipts
- **user-service**: Welcome email on signup
- **auth-service**: Password reset emails

## Template System

Notifications use Handlebars templates stored in **database-service** and cached in **cache-service**. Dynamic data (order #, customer name) is injected at send time.

## Delivery Guarantees

- Retry up to 5 times with exponential backoff
- Dead letter queue for failed notifications after retries
- Webhook notifications from **email-service** and **sms-service** track delivery

## Performance

- Send confirmation within 100ms
- Actual delivery to carrier: varies (email <5s, SMS <2s)
