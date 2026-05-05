# Start Work

Sync local `main` with `origin/main` before starting a new task.

**Trigger phrases:** 開工, "start work", or any clear equivalent ("let's start", "begin work"). When the user says one of these, follow the steps below.

## Commands (run in order)

```bash
git checkout main
git pull origin main
```

## Notes for the agent

- **Re-read this file each time.** Treat its current contents as the spec, not whatever was there last session — the user edits it as the workflow evolves.
- **Worktree caveat.** If the shell is in a sibling worktree (path contains `.claude/worktrees/<name>`), `git checkout main` will fail because `main` is already checked out in the primary worktree. Don't `cd` — instead resolve the primary worktree's path from `git worktree list` (the row whose path does **not** contain `.claude/worktrees/`) and run the same commands against it via `git -C <primary-path> ...`.
- **Pull aborted by untracked-file conflict.** If `git pull` reports *"untracked working tree files would be overwritten by merge"*, inspect the conflicting file first (e.g. `git diff --no-index <local-path> <(git show origin/main:<file>)`). Confirm with the user before removing or moving it — the local copy may be in-progress work that hasn't been committed yet.
- **Reporting.** After success, briefly tell the user the fast-forward range (e.g. `fa591ed..61fc6a7`, N commits) and the new HEAD commit. If anything was skipped, deferred, or required user intervention, mention it too.
