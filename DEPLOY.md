updated: 2026-05-13 12:00:00

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
7. **Wait for the `Build & push images` GitHub Actions run to finish green.** This is triggered by both the push to `main` (step 4) and the release (step 6); the release run produces the version-tagged image. Check with `gh run list --workflow=build-images.yml --limit=3`.
8. SSH deploy to GCP: `gcloud compute ssh gyeon --zone=asia-east1-b --command="cd /opt/gyeon && bash deploy.sh"`. The VM pulls `ghcr.io/eddytsoi/gyeon-{backend,frontend}:latest` and restarts — typically ~30–60 s, no compilation on the VM.

Next time you say "deploy", I'll follow this flow automatically. If you want a specific version (e.g. `v0.4.0`), just say so and I'll skip the auto-bump.

### Rollback

To pin to a previous version on the VM:

```
gcloud compute ssh gyeon --zone=asia-east1-b --command="cd /opt/gyeon && IMAGE_TAG=v0.9.67 docker compose -f docker-compose.prod.yml --env-file .env up -d"
```

Replace `v0.9.67` with any tag pushed to GHCR. Confirm available tags at <https://github.com/eddytsoi/gyeon/pkgs/container/gyeon-backend>.

### Pre-flight sanity (before bumping version)

Always run before drafting the bump commit:

```
git fetch origin && git log origin/main -3 --oneline && grep version frontend/package.json && gh release list --limit 5
```

Don't trust the session-start git status snapshot — parallel sessions may have shipped versions you didn't see. The latest `package.json` value plus the latest `gh release list` are the source of truth.

## One-time VM setup: GHCR auth

The VM needs to authenticate to GHCR before the first `docker compose pull`. Do this once per VM (and after PAT rotation):

1. Create a GitHub Personal Access Token (classic) with **only** the `read:packages` scope: <https://github.com/settings/tokens/new?scopes=read:packages&description=gyeon-vm-ghcr>.
2. SSH to the VM and log in:

   ```
   gcloud compute ssh gyeon --zone=asia-east1-b
   echo <PAT> | docker login ghcr.io -u eddytsoi --password-stdin
   ```

   Credentials persist in `~/.docker/config.json`. Rotate the PAT annually.
