# Developer Runbook

## Local API
1. Start Postgres and Docker (see `docker-compose.yml`).
2. Run the API:
```
go run ./cmd/api
```

## CLI
1. Create a demo key:
```
cloud auth create-demo my-user
```
2. Use CLI commands:
```
cloud compute list
```

## Web Dashboard
1. Run the web app from `web/`:
```
npm run dev
```
2. Open `http://localhost:3000` and set your API key in **Settings**.

## Common Issues
- **Unauthorized**: ensure `X-API-Key` is set or saved in Settings.
- **Docker errors**: confirm Docker daemon is running.
- **Database errors**: ensure Postgres is reachable and migrations succeed.
