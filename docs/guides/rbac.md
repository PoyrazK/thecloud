# Role-Based Access Control (RBAC)

This guide explains the RBAC system introduced in v0.3.0: roles, permissions, APIs, and examples for typical workflows.

## Overview

RBAC provides a way to assign permissions to users via roles. The system supports:

- Roles: collections of permissions (e.g., `admin`, `developer`, `viewer`)
- Permissions: fine-grained actions on resources (e.g., `instances.create`, `volumes.snapshot`)
- Role bindings: assign a role to a user or service account

## Quick Start (CLI)

Create a role (example):

```bash
# Create a role via CLI (example)
cloud roles create --name developer --permissions "instances.create,instances.start,volumes.attach"
```

Bind role to a user:

```bash
cloud roles bind --role developer --user user@example.com
```

## API Examples

- `POST /rbac/roles` — Create role
- `GET /rbac/roles` — List roles
- `POST /rbac/bindings` — Create role binding
- `GET /rbac/bindings` — List bindings

Example: create role via curl

```bash
curl -X POST http://localhost:8080/rbac/roles \
  -H "X-API-Key: $API_KEY" \
  -d '{"name":"viewer","permissions":["instances.view","volumes.view"]}'
```

## Best Practices

- Use least privilege when assigning roles
- Create service accounts for automation with limited roles
- Rotate API keys regularly

## Troubleshooting

- 403 Forbidden on protected endpoints: verify role bindings and permission names
- Audit logs: check audit table for role assignment changes

## Next Steps

- Link RBAC to UI: update console to manage roles and bindings
- Add migration docs for upgrading older installations
