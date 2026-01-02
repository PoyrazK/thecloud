# Contributing to The Cloud

Thanks for taking the time to contribute! This guide keeps changes consistent and easy to review.

## Quick Start
- Fork and create a feature branch from `main`.
- Keep changes scoped and focused.
- Add or update tests when behavior changes.
- Run checks before opening a PR:
  - `go test ./...`
  - `golangci-lint run` (or the CI lint job)
  - `cd web && npm run lint && npm run build`

## Pull Requests
- Describe the problem and the approach.
- Link related issues or docs.
- Include screenshots or logs when relevant.
- Avoid formatting-only changes unless necessary.

## Code Style
- Go: follow `gofmt` and project conventions.
- Frontend: follow existing ESLint rules.
- Keep configs and docs aligned with code changes.

## Security
If you find a security issue, please follow `SECURITY.md` instead of opening a public issue.
