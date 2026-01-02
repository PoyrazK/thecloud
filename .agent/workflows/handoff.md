---
description: Resume development after machine transition
---
# 🔄 Agent Handoff: Initializing State

If you are a new agent picking up this project on a new machine, follow these steps to synchronize your internal state with the previous session.

## 1. Sync Task State
Read the current task list from `.agent/handoff/current_tasks.md` and use it to update your own `task.md` artifact.

## 2. Sync Plan State
Read the current implementation plan from `.agent/handoff/current_plan.md` to understand what was mid-process.

## 3. Understand Recent Progress
Review the latest walkthrough at `.agent/handoff/latest_walkthrough.md` for a summary of completed and verified features.

## 4. Technical Strategy
Refer to `docs/FUTURE_ROADMAP.md` for the long-term plan (Phases 6-10) and the specific technical constraints (e.g., Database Port 5433).

## 📊 Quick Start Commands
```bash
# Verify backend state
make run
# Check health
curl http://localhost:8080/health
# Run tests
go test ./... -v
```

## ⚠️ Important Rules for this Project
- **NEVER** push to GitHub without asking the USER first.
- **NEVER** mention "Phase X" or "Sprint Y" in commit messages. Keep them strictly technical.
- **ALWAYS** check `internal/platform/config.go` for the latest environment configuration.
