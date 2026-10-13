# Runbook: deploying mykomora on one small VPS

Target: one Hetzner Cloud VPS (2 GB RAM, Helsinki), a free DuckDNS hostname,
Cloudflare R2 for photos and backups, about €4–5 a month. The infrastructure is
OpenTofu (`infra/`); deploys are GitHub Actions (`.github/workflows/deploy.yml`)
running `deploy/deploy.sh` on the server.

## How a deploy works

```
push to main -> CI green -> Deploy workflow
   build core-api, migrate, web images -> ghcr.io/olehmushka/mykomora-*:sha-<7>
   scp deploy/ files to the server
   ssh deploy@server deploy/deploy.sh sha-<7>
        pull images (old version keeps serving)
        run migrations in a one-shot container (failure stops the deploy here)
        recreate core-api + web
        wait for both to answer; if not, put the previous tag back
   smoke test https://<domain>/healthz
```

**Availability.** There is one replica of each service. While a container is
replaced, Caddy retries the upstream for up to 15 s (`lb_try_duration`), so
requests wait a moment instead of failing. For a family-sized load that is
invisible. It is not a true zero-downtime rollout; if that ever matters, run two
replicas of core-api and web behind Caddy (costs roughly 200 MB more RAM).

**Migration rule (important).** Migrations run before the new code starts, and a
rollback does not undo them. So every migration must leave the *previous*
release working: add columns and tables first; drop or rename only in a later
release, once nothing reads the old shape. A destructive migration in the same
release as the code that stops needing it is the one way a "safe" rollback
breaks.

## 1. One-time setup

1. **Infra:** follow `infra/README.md`. Result: server, firewall, buckets, CORS,
   backup retention and R2 credentials.
2. **DuckDNS:** point `yourname.duckdns.org` at `tofu output server_ipv4`. The
   address is a primary IP, so it survives server rebuilds. Check with
   `dig +short yourname.duckdns.org` before the first deploy, or certificate
   issuance fails.
3. **Secrets on the server.** The only file that lives only on the server:
   ```
   ssh admin@<ip>
   # .env.prod.example arrives with the first CI sync; until then scp deploy/.env.prod.example there
   cp /opt/mykomora/deploy/.env.prod.example /opt/mykomora/deploy/.env.prod
   chmod 600 /opt/mykomora/deploy/.env.prod && chown deploy: /opt/mykomora/deploy/.env.prod
   ```
   Fill every empty value (generators are in the file). `S3_*` values come from
   `tofu output`.
4. **GitHub.** Repo > Settings:
   - Environment `production` (optionally with a required reviewer).
   - Secrets: `DEPLOY_HOST` (the IP or hostname), `DEPLOY_SSH_KEY` (private half
     of the CI key), `DEPLOY_KNOWN_HOSTS` (output of `ssh-keyscan <ip>` run from
     a machine you trust, after the server is up).
   - Variable: `DEPLOY_DOMAIN` (`yourname.duckdns.org`, for the smoke test).
5. **First deploy.** Run the Deploy workflow by hand (Actions > Deploy > Run
   workflow). It pushes the images; then in GitHub > Packages make
   `mykomora-core-api`, `mykomora-migrate` and `mykomora-web` **public** (new
   packages default to private) and re-run the failed deploy job. Afterwards
   every push to `main` deploys by itself.
6. **Google sign-in.** OAuth client redirect URI exactly
   `https://yourname.duckdns.org/api/v1/auth/google/callback`. Keep the consent
   screen in *Testing* and add each family member as a test user. Try a real
   login; if Google rejects the DuckDNS host, switch to a cheap domain.
7. **Backups.** On the server, as `deploy`: `crontab -e`, add
   `17 3 * * * /opt/mykomora/deploy/backup.sh >> /var/log/mykomora-backup.log 2>&1`
   (create the log file owned by `deploy`, or log to `~/backup.log`). Run
   it once by hand and confirm an object appears in the backup bucket.

## 2. Everyday operations

- **Deploy:** merge to `main`. Watch Actions.
- **Roll back:** Actions > Deploy > Run workflow with the commit SHA you want, or
  on the server `deploy/deploy.sh "$(cat deploy/.previous-tag)"`.
- **Status / logs:** on the server,
  `docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml ps`
  (append `logs -f core-api`, `web`, `caddy` or `postgres`).
- **Change the compose file, Caddyfile or scripts:** commit them; they are copied
  on every deploy. Only `.env.prod` is edited on the server (then run
  `deploy.sh "$(cat deploy/.current-tag)"` to apply it).
- **Rebuild the server:** `tofu apply -replace=hcloud_server.app`, restore the
  latest dump (below), copy `.env.prod` back, run the Deploy workflow.

## 3. Backups

`backup.sh` dumps Postgres nightly to the backup bucket; the OpenTofu lifecycle
rule deletes dumps after 14 days. Photos and attachments already live in R2 and
are not copied.

### Restore rehearsal (do once before relying on it, and after big changes)
```
# 1. fetch the latest dump
docker run --rm -v "$PWD:/b" -e AWS_ACCESS_KEY_ID=... -e AWS_SECRET_ACCESS_KEY=... \
  -e AWS_DEFAULT_REGION=auto amazon/aws-cli:2 --endpoint-url "$S3_ENDPOINT" \
  s3 cp s3://mykomora-backups/postgres/<file>.dump /b/restore.dump

# 2. restore into a scratch database
C="docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml"
$C exec -T postgres createdb -U mykomora restore_test
$C exec -T postgres pg_restore -U mykomora -d restore_test --no-owner < restore.dump

# 3. sanity check, then drop it
$C exec -T postgres psql -U mykomora -d restore_test -c 'select count(*) from items'
$C exec -T postgres dropdb -U mykomora restore_test
```
To restore for real: `$C stop core-api web`, drop and recreate the `mykomora`
database, `pg_restore` into it, then run `deploy.sh "$(cat deploy/.current-tag)"`.

## 4. Sizing notes
- Measured about 4.8 KB of Postgres per item (events, photo rows and indexes
  included): 20,000 items is about 105 MB.
- Memory limits in the compose file add up to about 1 GB, leaving the rest for
  the OS and page cache on a 2 GB host. Nothing is built on the server any more.
