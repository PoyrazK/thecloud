# Stacks

Stacks are a convenience layer that groups resources (instances, networks, volumes, LB) and deploys them as a single unit.

## Concepts

- Stack: named collection of resources
- Template: YAML or JSON file declaring resources
- Stack lifecycle: create -> apply -> update -> destroy

## Quick CLI

Create/apply a stack:

```bash
cloud stacks apply -f my-stack.yaml
```

List stacks:

```bash
cloud stacks list
```

Destroy a stack:

```bash
cloud stacks destroy --name my-stack
```

## API Endpoints

- `POST /stacks` — Create stack
- `GET /stacks` — List stacks
- `POST /stacks/:id/apply` — Apply stack
- `DELETE /stacks/:id` — Delete stack

## Notes

- Stacks are useful for reproducible environments and CI workflows
- Stacks should be idempotent; applying the same template twice should converge to the same state

## Next Steps

- Add template examples for multi-tier apps
- Add rollback support for failed applies
