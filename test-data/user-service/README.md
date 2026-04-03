# User Service

## Overview

User account management and profile service. Handles user registration, profile updates, account deletion, and user metadata. Single source of truth for user identity across the platform.

## Responsibilities

- User registration and onboarding
- Profile management (name, email, phone, preferences)
- Account closure and data deletion
- User search and discovery
- Profile image management

## Key Dependencies

- **auth-service**: Validates user credentials during signup; used for permission checks
- **database-service**: Persistent storage of user profiles and metadata
- **cache-service**: Caches active user profiles for fast lookup
- **notification-service**: Sends welcome emails on signup, password reset confirmations
- **search-service**: Indexes user profiles for discovery features
- **logging-service**: Audit trail for account creation/deletion

## Integration Points

Called by:
- **order-service**: Customer lookup when placing orders
- **recommendation-service**: User history and preferences for recommendations
- **api-gateway-service**: User profile endpoint

## Data Model

```
User {
  id: UUID
  email: string (unique)
  name: string
  phone: string
  preferences: JSON (theme, notifications, etc)
  created_at: timestamp
  updated_at: timestamp
}
```

## Caching Strategy

Active user profiles cached in **cache-service** with 5-minute TTL. Cache invalidated on any profile update.
