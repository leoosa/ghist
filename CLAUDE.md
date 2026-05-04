<!-- ghist:start -->
## Ghist — Project Memory

This project uses [ghist](https://github.com/unnecessary-special-projects/ghist) for persistent project state.

**Required:** Run `ghist status` at the start of every session.

Tasks are addressable by their RefID (`GHST-xxxxxxxx` — printed by `ghist task add`), full UUID, filename slug, or any unique UUID prefix. The `<ref>` placeholder below accepts any of those forms.

### Workflow Rules
1. **Save plans to the task as soon as they're ready:** `cat <<'EOF' | ghist task update <ref> --plan-stdin`
2. **Keep the plan current** — update it on every meaningful change as work progresses.
3. **Write implementation notes on the task** when work is complete — append a `## Implementation Notes` section to the plan via `--plan-stdin`.
4. **Ask the user before closing** a task — never auto-close.

### Quick Reference
- `ghist task list` — see all tasks
- `ghist task add "title"` — create a task (prints the RefID; capture with `TASK_REF=$(ghist task add "..." | awk '{print $3}' | tr -d :)` to reuse downstream)
- `ghist task update <ref> --status in_progress` — update status
- `ghist task update <ref> --plan-stdin` — save or update a plan (pipe via stdin)
- `ghist log "message" --task <ref>` — record a decision or note, optionally linked to a task
- `ghist skills show <name>` — read detailed skill instructions

### Available Skills
- `ghist skills show context-sync` — session start/end protocol
- `ghist skills show task-workflow` — find → plan → execute → complete loop (statuses: todo, in_planning, in_progress, done, blocked)
- `ghist skills show auto-completion` — auto-detect task completion
- `ghist skills show log-thinking` — log decisions and reasoning
- `ghist skills show commit-link` — link git commits to tasks automatically
<!-- ghist:end -->
