# Authentication Guide

## Overview

The Cloud implements a secure, multi-tenant authentication system using API Keys. This ensures resource isolation, meaning users can only access resources (instances, VPCs, volumes, etc.) that they have created.

## User Registration

Before using the API, you must register a new account.

### Endpoint
`POST /auth/register`

### Request
```json
{
  "email": "user@example.com",
  "password": "securepassword123",
  "name": "John Doe"
}
```

### Response
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "John Doe",
    "role": "user"
  }
}
```

---

## Login & API Key Retrieval

After registering, log in to obtain your API Key. This key must be included in all subsequent API requests.

### Endpoint
`POST /auth/login`

### Request
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

### Response
```json
{
  "data": {
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "name": "John Doe",
      "role": "user"
    },
    "api_key": "thecloud_123456789..."
  }
}
```
*Note: The `api_key` field contains your API Key.*

---

## Authenticating Requests

All protected API endpoints require the `X-API-Key` header.

### Header Format
```
X-API-Key: apk_123456789...
```

### Example Request
```bash
curl -H "X-API-Key: apk_123456789..." http://localhost:8080/instances
```

If the key is missing or invalid, the API will return `401 Unauthorized`.

---

## WebSocket Authentication

For WebSocket connections (e.g., real-time metrics), the API Key is passed as a query parameter.

### URL Structure
`ws://localhost:8080/ws?api_key=apk_123456789...`

---

## RBAC (Role-Based Access Control)

RBAC restricts access by role. Available roles:
- `owner`
- `admin`
- `developer`
- `viewer`

### Inspect Your Role
`GET /auth/me/role`

### List Roles
`GET /auth/roles` (requires `auth:read`)

### Update a User Role
`PUT /auth/users/{id}/role` (requires `auth:update`)

---

## Security Best Practices

1. **Keep your API Key secret:** Do not commit it to version control or share it publicly.
2. **Use HTTPS:** In production, ensure all API traffic is encrypted.
3. **Rotate Keys:** If you suspect your key has been compromised, generate a new one (feature coming soon).
