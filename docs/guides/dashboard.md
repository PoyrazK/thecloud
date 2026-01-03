# Dashboard Guide

## Overview
The dashboard provides read-only visibility into core resources (instances, events, storage objects, VPCs). It reads data from the API and requires an API key.

## Configure API Access
1. Start the API server on `http://localhost:8080`.
2. Open the web app and go to **Settings**.
3. Paste your API key and click **Save API Key**.

Optional: set a custom API URL before building the web app.
```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Read-only Pages
- Dashboard summary and recent events
- Compute instances
- Activity events
- Network VPCs
- Storage objects by bucket name

## Notes
- Actions are currently disabled to keep the UI read-only.
- The storage page expects an existing bucket name; it lists objects in that bucket.
