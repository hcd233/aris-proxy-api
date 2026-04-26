---
name: deploy-to-production
description: Build and deploy aris-proxy-api to production. Use this skill whenever the user wants to deploy, ship, release, or push to production. Covers the full pipeline: git commit + push → wait for GitHub Actions docker build → SSH to server and run deploy script.
compatibility: Requires gh CLI authenticated, SSH key for production server, and docker compose on the server.
---

# deploy

Deploy the aris-proxy-api project to the production server at `api.lvlvko.top`.

## Workflow

This skill covers the full pipeline — commit and push, wait for CI build, then deploy on server.

### Step 1: Commit and push

If there are uncommitted changes, stage and commit them first. Ask the user for a commit message if one is not provided. Use conventional commit format: `type(scope): description`.

```bash
git add -A
git commit -m "<message>"
git push origin <current-branch>
```

After pushing, note the current branch name — it will be needed for the image tag.

### Step 2: Wait for Docker build

The GitHub Actions workflow `docker-publish.yml` triggers on push to `master` or version tags (`v*.*.*`). If the push is to a non-master branch, the workflow may not trigger automatically — adjust the tag approach accordingly.

Find and watch the workflow run:

```bash
# List recent runs for the docker-publish workflow
gh run list --workflow docker-publish.yml --repo hcd233/aris-proxy-api --limit 5

# Watch the latest run (replace <run-id> with the actual ID)
gh run watch <run-id> --repo hcd233/aris-proxy-api
```

Poll until the build completes. The build usually takes under 3 minutes. If `gh run watch` exits with a non-zero status, the build failed — report this to the user.

### Step 3: Deploy on server

Once the Docker image is built and pushed to GHCR, resolve the production server IP from the domain and SSH into it:

```bash
# Resolve the server IP from the production domain
PROD_IP=$(dig +short api.lvlvko.top | head -1)

# SSH into the server and run the deploy script
ssh ubuntu@${PROD_IP} 'cd code/aris-proxy-api/ && bash script/deploy.sh'
```

The deploy script:
1. `git fetch` + `git pull --ff-only` — pulls latest code for docker-compose config
2. `docker pull ghcr.io/hcd233/aris-proxy-api:<branch>` — pulls the new image
3. `docker compose up -d` — restarts the service with the new image
4. `docker image prune -a -f` — cleans up old images
5. `docker logs -f aris-proxy-api --details` — tails logs to verify the service started correctly

**Important**: The last step (`docker logs -f`) will follow logs indefinitely. After a few seconds of healthy-looking logs, the deployment is successful. Press Ctrl+C to stop tailing.

### Step 4: Report

After deployment, summarize:
- Branch and commit deployed
- Build status and duration
- Any warnings or errors from the logs

## Server details

| Item | Value |
|------|-------|
| Domain | `api.lvlvko.top` |
| Resolve | `dig +short api.lvlvko.top` |
| User | `ubuntu` |
| Project path | `code/aris-proxy-api/` |
| Deploy script | `script/deploy.sh` |
| Docker image | `ghcr.io/hcd233/aris-proxy-api` |
| Service name | `aris-proxy-api` |

## Troubleshooting

- **Build not triggering**: The workflow only triggers on `master` and `v*.*.*` tags. For other branches, push to master or create a version tag.
- **Build failed**: Use `gh run view <run-id> --log --repo hcd233/aris-proxy-api` to inspect logs.
- **SSH fails**: Check that the SSH key is loaded (`ssh-add -l`).
- **docker pull fails**: Verify the image tag matches the branch name (with `/` replaced by `-`).
- **Container crashes**: After deployment, check logs via `ssh ubuntu@$(dig +short api.lvlvko.top | head -1) 'docker logs aris-proxy-api --tail 50'`.
