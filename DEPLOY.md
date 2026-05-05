updated: 2026-05-05 20:45:00

## Deploy Flow

1. Work on a worktree branch (`claude/<name>` or `chore/<name>`) and commit the feature/fix.
2. **Bump the version** in `frontend/package.json` (auto-bump patch by default, e.g. `0.9.5 → 0.9.6`; if the user specified a version, use that) **as its own commit** on the same branch. Commit title format must be exactly:

   ```
   chore: bump version to vX.Y.Z
   ```

   This commit must contain only the `package.json` (and `package-lock.json` if it also moved) version line — no other changes — so the bump is auditable in the PR diff even after squash.

3. Open a PR against `main` containing both the feature commit(s) and the version-bump commit.
4. Claude self-reviews the PR and squash-merges it into `main` (no separate approval step).
5. Push is implicit after squash-merge — verify `git log origin/main -1` shows the merge commit.
6. `gh release create vX.Y.Z --target <merge-sha> --generate-notes` (or with custom notes).
7. SSH deploy to GCP: `gcloud compute ssh gyeon --zone=asia-east1-b --command="cd /opt/gyeon && bash deploy.sh"`.

Next time you say "deploy", I'll follow this flow automatically. If you want a specific version (e.g. `v0.4.0`), just say so and I'll skip the auto-bump.

### Pre-flight sanity (before bumping version)

Always run before drafting the bump commit:

```
git fetch origin && git log origin/main -3 --oneline && grep version frontend/package.json && gh release list --limit 5
```

Don't trust the session-start git status snapshot — parallel sessions may have shipped versions you didn't see. The latest `package.json` value plus the latest `gh release list` are the source of truth.
