# Infrastructure as Code (IaC)

This guide introduces the minimal IaC support in v0.3.0 and how to use `cmd/cloud/iac.go` and stacks to provision resources declaratively.

## Overview

The project includes a simple declarative format (YAML) for creating groups of resources (stacks). IaC supports:

- Defining instances, volumes, networks, and load balancers
- Applying stacks via CLI or API
- Preview (dry-run) mode to validate templates

## Example Stack (YAML)

```yaml
name: example-app
resources:
  - type: network
    name: app-vpc
  - type: instance
    name: web-1
    image: ubuntu-22.04
    network: app-vpc
  - type: volume
    name: web-data
    size_gb: 10
```

Apply a stack via CLI:

```bash
cloud iac apply -f stack.yaml
```

API:

- `POST /iac/validate` — Validate template
- `POST /iac/apply` — Apply stack
- `GET /iac/stacks/:id` — Get stack status

## Best Practices

- Keep stacks small and composable
- Use variables for environment-specific values
- Track IaC files in Git and use CI to validate templates

## Next Steps

- Add examples for multi-instance deployments
- Integrate with webhook-driven CI/CD
