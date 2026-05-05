updated: 2026-05-05 20:30:00

## Deploy Flow

1. Work on a worktree branch (`claude/<name>` or `chore/<name>`).
2. Open a PR against `main`. If no version number was specified, auto-bump the patch version (e.g. `v0.9.5 → v0.9.6`) by editing `frontend/package.json` in the same PR.
3. Squash-merge the PR into `main` (Claude reviews and merges automatically — no extra approval step).
4. Push merge commit to GitHub (already on `origin/main` after squash).
5. `gh release create vX.Y.Z --target <merge-sha> --generate-notes` (or with custom notes).
6. SSH deploy to GCP: `gcloud compute ssh gyeon --zone=asia-east1-b --command="cd /opt/gyeon && bash deploy.sh"`.

Next time you say "deploy", I'll follow this flow automatically. If you want a specific version (e.g. `v0.4.0`), just say so and I'll skip the auto-bump.

### Pre-flight sanity (before bumping version)

Always run before drafting the PR:

```
git fetch origin && git log origin/main -3 --oneline && grep version frontend/package.json
```

Don't trust the session-start git status snapshot — parallel sessions may have shipped versions you didn't see. The latest `package.json` value plus the latest `gh release list` are the source of truth.
