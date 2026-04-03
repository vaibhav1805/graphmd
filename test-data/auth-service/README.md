# Auth Service

## Overview

Centralized authentication and authorization service. Handles user login, session management, JWT token generation, and permission validation. All authentication decisions in the platform go through this service.

## Responsibilities

- User login and logout
- JWT token generation and validation
- Session token lifecycle management
- Permission and role-based access control (RBAC)
- OAuth2 integration for third-party apps

## Key Dependencies

- **user-service**: User profile lookup and account verification
- **database-service**: Stores credentials and permission mappings
- **cache-service**: Session token caching for fast validation (sub-ms response times)
- **security-service**: Vulnerability scanning and threat detection
- **logging-service**: Audit logs for all authentication events

## Integration Points

The auth-service is called by:
- **api-gateway-service**: On every request for JWT validation
- **user-service**: For credential verification during signup
- **security-service**: For policy enforcement

## Token Strategy

Tokens are stored in the **cache-service** with 1-hour TTL. The database-service maintains a permanent audit log of all token issues/revocations.

## Security

- Passwords hashed with bcrypt (cost factor 12)
- Rate limiting on login attempts (5 attempts per minute per IP)
- Session invalidation on password change
