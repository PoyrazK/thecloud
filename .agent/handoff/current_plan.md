# Phase 5: The Console - Implementation Plan (Handoff)

This plan outlines the technical strategy for the Console feature.

## 🏛️ Recent Progress
- **Backend API**: Finished `/summary`, `/events`, `/stats`.
- **Infrastructure**: Added auto-migrator for new environments.
- **Port Strategy**: DB is on **5433**.

## 🎨 Next Steps (Sprint 3: Frontend)

### Project Init
```bash
npx -y create-next-app@14 frontend --typescript --tailwind --app --src-dir
```

### Key Integrations
1. **API Client**: Axios or Fetch wrapper pointing to `PROCESS.ENV.NEXT_PUBLIC_API_URL` (usually `http://localhost:8080`).
2. **SSE Hook**: Listen for `summary` events to update the Resource Cards.
3. **WS Connection**: Listen for `INSTANCE_CREATED/STOPPED` events for the Activity Feed.

### Critical Focus
- Ensure the frontend respects the `miniaws_` API key auth.
- Use a dark, premium theme (Tailwind `slate` or `zinc`).
