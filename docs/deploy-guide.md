# Putting mykomora online: a step-by-step guide

For people who are comfortable following instructions but are not server
experts. Every step says where to click or what to type. The short technical
version is `docs/runbook.md`; this guide is the long, gentle one.

**What you will end up with:** the app running at `https://your-address`, on a
small rented server in Finland, photos stored in Cloudflare, a nightly backup,
and every change you merge to `main` going live by itself in a couple of
minutes.

**Time:** about 2–3 hours the first time (most of it clicking through sign-ups).
**Cost:** about €4–5 per month for the server. Everything else is free, or
about €10 per year if you buy your own domain.

---

## Part 0. The big picture

| Piece | What it is | Who provides it | Cost |
|---|---|---|---|
| Server | A small computer in a data centre that runs the app | Hetzner | ~€4–5 / month |
| Address | The name people type, e.g. `mykomora.duckdns.org` | DuckDNS (free) or a domain you buy | free / ~€10 a year |
| Photo storage | Where uploaded photos and files live | Cloudflare R2 | free up to 10 GB |
| Sign-in | "Sign in with Google" | Google Cloud | free |
| Deploy robot | Builds and ships your code on every merge | GitHub Actions | free |
| OpenTofu | A tool on your laptop that creates the server and firewall from the files in `infra/`, so you never click through the Hetzner console to build them | open source | free |

Prices and screens change from time to time. If a button is named slightly
differently, look for the closest match.

---

## Part 1. Decide your address (domain)

The app needs a proper web address. A bare IP address will not work: Google
sign-in refuses it, and the barcode scanner needs a secure (https) address.

**Option A, free: DuckDNS (recommended to start).**
You get a name like `mykomora.duckdns.org`. Nothing to pay, nothing to renew.
Do it in Part 7, once you know the server's IP. For now just choose the name and
write it down.

**Option B, buy your own domain (about €10 a year).**
A short name of your own, e.g. `mykomora.com`. Looks nicer, and some people
find it more dependable than a free service. Where to buy:
- **Cloudflare Registrar** (dash.cloudflare.com → Domain Registration → Register
  Domains). Sells at cost with no markup, and you already need a Cloudflare
  account for storage. Usually the cheapest for `.com`.
- Any other registrar (Namecheap, Porkbun, …) is fine too.
Unusual endings (`.xyz`, `.click`) are often cheaper than `.com` in year one.
Check the **renewal** price, not just the first-year price.

You can start with DuckDNS and move to a bought domain later. It means
changing the address in a few places (Google settings, `.env.prod`, one GitHub
variable).

> Throughout this guide, **`YOUR-ADDRESS`** means the full name you chose,
> for example `mykomora.duckdns.org`.

---

## Part 2. Create the accounts you need

Do these in a browser. Use one password manager to keep every password, token
and key in this guide: you will create about ten secrets and some are shown
only once.

### 2.1 GitHub
You already have the repository. Nothing to do yet.

### 2.2 Hetzner (the server)
1. Go to **hetzner.com/cloud** → Sign up. They may ask for ID verification or a
   small payment check; this is normal and can take a few hours.
2. Once in, click **New project**, name it `mykomora`.
3. In the project: **Security → API tokens → Generate API token**. Name it
   `opentofu`, permission **Read & Write**. **Copy the token now** (it is shown
   once) and save it in your password manager. Call it **HETZNER TOKEN**.

### 2.3 Cloudflare (photo storage)
1. **dash.cloudflare.com** → Sign up (free plan).
2. In the left menu open **R2 Object Storage**. Cloudflare asks you to add a
   payment method to enable R2. The free allowance (10 GB stored, generous
   request limits) covers this app; you are charged only if you go over it.
   Confirm activation.
3. Note your **Account ID**: on the R2 page it is shown on the right side (a
   32-character code). Save it. Call it **ACCOUNT ID**.

### 2.4 Google (the sign-in button)
Done later in Part 8, once you know your address.

---

## Part 3. Install the tools on your laptop

You need two things: **OpenTofu** and **ssh-keygen** (already on Linux/macOS;
on Windows use WSL or Git Bash).

OpenTofu: follow the install page at **opentofu.org/docs/intro/install**, then
check it works:

```
tofu version
```

You should see a version number (1.8 or newer).

---

## Part 4. Create two SSH keys

An SSH key is a pair of files: a **private** one (secret, never shared) and a
**public** one (safe to hand out). You need two pairs: one for **you** to log
into the server, one for the **deploy robot**.

Open a terminal in any folder you will remember (e.g. `~/mykomora-keys`) and
run:

```
mkdir -p ~/mykomora-keys && cd ~/mykomora-keys
ssh-keygen -t ed25519 -f admin_key  -N '' -C "mykomora admin"
ssh-keygen -t ed25519 -f deploy_key -N '' -C "mykomora deploy robot"
```

You now have four files:
- `admin_key` (private) and `admin_key.pub` (public): for you
- `deploy_key` (private) and `deploy_key.pub` (public): for the robot

Back them up in your password manager. **Never commit these to git.**

---

## Part 5. Prepare Cloudflare storage for OpenTofu

OpenTofu has to remember what it built (its "state"). It keeps that in a small
private Cloudflare bucket. OpenTofu builds the Hetzner server and firewall; the
Cloudflare buckets for photos and backups you create by hand in Part 6 (it takes
a few minutes and avoids a lot of token trouble).

### 5.1 The state bucket
R2 Object Storage → **Create bucket** → name `mykomora-tfstate` → Create.

### 5.2 A key that can only touch that bucket
R2 → **Manage R2 API Tokens** (right side) → **Create API token**.
- Permission: **Object Read & Write**
- Specify bucket: **mykomora-tfstate** only
- Create. Cloudflare shows an **Access Key ID** and a **Secret Access Key**.
  Save both. Call them **STATE KEY ID** and **STATE KEY SECRET**.

### 5.3 A passphrase that locks the state
Make up a long random sentence or run `openssl rand -base64 24`. This encrypts
OpenTofu's notes. **If you lose it, OpenTofu's state becomes unreadable** (you
would have to import everything again by hand). Save it. Call it **STATE
PASSPHRASE**.

---

## Part 6. Create the server (OpenTofu) and the photo storage

In a terminal, inside your copy of the repository:

```
cd infra
cp backend.hcl.example backend.hcl
cp terraform.tfvars.example terraform.tfvars
```

Open **backend.hcl** and fill in the account ID in the endpoint:

```
bucket = "mykomora-tfstate"
endpoints = {
  s3 = "https://<ACCOUNT ID>.r2.cloudflarestorage.com"
}
```

Open **terraform.tfvars** and fill in:
- `admin_ssh_public_key`: paste the **entire contents** of `~/mykomora-keys/admin_key.pub`
- `deploy_ssh_public_key` → the entire contents of `deploy_key.pub`

(Neither file contains secrets: both are meant to be public.)

Now give OpenTofu the secrets. These lines only last for this terminal window,
and nothing is written to disk:

```
export AWS_ACCESS_KEY_ID='<STATE KEY ID>'
export AWS_SECRET_ACCESS_KEY='<STATE KEY SECRET>'
export TF_VAR_hcloud_token='<HETZNER TOKEN>'
export TF_VAR_state_passphrase='<STATE PASSPHRASE>'
```

(`AWS_...` is just the name OpenTofu uses for any S3-style storage; here it
means Cloudflare.)

Then:

```
tofu init -backend-config=backend.hcl
tofu plan
```

`plan` is a preview: it lists what will be created and changes nothing. Read it.
You should see four things: an SSH key, an IP address, a firewall and a server. If it shows an error
about the **server type**, run the Hetzner CLI or check the Hetzner price list
for the cheapest 2 GB "shared vCPU" plan and set `server_type` in
`terraform.tfvars`.

When the preview looks right:

```
tofu apply
```

Type `yes`. After a minute or two it prints your outputs. Write these down:

```
tofu output server_ipv4                  # the server's address, e.g. 95.216.1.2
```

### Create the photo and backup storage (by hand, about 5 minutes)
In Cloudflare → **R2 Object Storage**:

1. **Create bucket** `mykomora-media` (location: Western Europe). Then create
   `mykomora-backups` the same way.
2. **mykomora-media → Settings → CORS Policy → Add**, and paste (replace
   YOUR-ADDRESS):
   ```json
   [{"AllowedOrigins":["https://YOUR-ADDRESS"],"AllowedMethods":["GET","PUT"],"AllowedHeaders":["*"],"ExposeHeaders":["ETag"],"MaxAgeSeconds":3600}]
   ```
   This lets the browser upload photos directly to the bucket.
3. **mykomora-backups → Settings → Object lifecycle rules → Add rule**: prefix
   `postgres/`, delete objects 14 days after they are uploaded. This keeps two
   weeks of backups and removes older ones.
4. **Storage address.** In either bucket's **Settings** find **S3 API** and copy
   the address. It looks like `https://<long-id>.r2.cloudflarestorage.com`; copy
   only that part, without the bucket name after it. Call it **S3 ENDPOINT**.

### Create the app's storage key (by hand, 2 minutes)
The app needs its own key to read and write photos:
1. R2 → **Manage R2 API Tokens → Create Account API token**.
2. Permission: **Object Read & Write**. Under "Specify bucket(s)" choose
   **mykomora-media** and **mykomora-backups** (both).
3. Create. Cloudflare shows an **Access Key ID** and a **Secret Access Key**
   (shown once). Save both in your password manager. Call them **APP KEY ID**
   and **APP KEY SECRET**.

The server needs another 2–3 minutes after this to finish installing itself.
Test that you can get in:

```
ssh -i ~/mykomora-keys/admin_key admin@<server_ipv4>
```

Answer `yes` to the fingerprint question. If you get a prompt like
`admin@mykomora:~$`, it worked; type `exit`. If the connection is refused, wait
a couple of minutes and retry.

---

## Part 7. Point your address at the server

### Option A: DuckDNS
1. Go to **duckdns.org** and sign in (Google or GitHub).
2. In **domains**, type your chosen name (e.g. `mykomora`) → **add domain**.
3. In the row for it, paste `<server_ipv4>` into the **current ip** box →
   **update ip**.

### Option B: a domain you bought on Cloudflare
1. dash.cloudflare.com → your domain → **DNS → Records → Add record**.
2. Type **A**, Name `@` (or a word like `app` for `app.yourdomain.com`),
   IPv4 address `<server_ipv4>`.
3. Set the cloud icon to **grey: DNS only** (not orange). The server fetches its
   own security certificate and needs to be reached directly.

### Check it (both options)
Wait a few minutes, then:

```
dig +short YOUR-ADDRESS
```

It must print your server's IP. Do not continue until it does; the first
secure-certificate request fails otherwise.

---

## Part 8. Set up "Sign in with Google"

1. Go to **console.cloud.google.com**. Create a project (top bar → New project →
   `mykomora`).
2. Menu → **APIs & Services → OAuth consent screen** (or "Google Auth
   Platform"). Choose **External**, fill in the app name `mykomora` and your
   email. Leave it in **Testing**.
3. In the same section, find **Test users** → **Add users** → enter the Google
   email of every family member who will sign in (you included). Only listed
   people can sign in while the app is in Testing, which is what you want. Up to
   100 are allowed.
4. **Credentials → Create credentials → OAuth client ID** → type **Web
   application**. Under **Authorised redirect URIs** add exactly:

   ```
   https://YOUR-ADDRESS/api/v1/auth/google/callback
   ```
5. Create. Google shows a **Client ID** and a **Client secret**. Save both.

If Google refuses your DuckDNS address as a redirect, buy a domain (Part 1,
Option B) instead and use that.

---

## Part 9. Fill in the server's private settings

The server keeps one private file with all its secrets. On your laptop, in the
repository:

```
cp deploy/.env.prod.example deploy/.env.prod
```

(That file is ignored by git, so it can't be committed by accident.) Open
`deploy/.env.prod` and fill in every empty value:

| Setting | What to put |
|---|---|
| `DOMAIN` | YOUR-ADDRESS (no `https://`) |
| `ACME_EMAIL` | your email (certificate expiry notices go here) |
| `POSTGRES_PASSWORD` | run `openssl rand -hex 24` and paste the result |
| `APP_SECRET` | run `openssl rand -base64 48` and paste the result |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | from Part 8 |
| `S3_ENDPOINT` | **S3 ENDPOINT** (Part 6) |
| `S3_ACCESS_KEY_ID` | **APP KEY ID** (Part 6) |
| `S3_SECRET_ACCESS_KEY` | **APP KEY SECRET** (Part 6) |

Leave `S3_REGION=auto`, the bucket names and the rest as they are.

Copy it to the server:

```
scp -i ~/mykomora-keys/admin_key deploy/.env.prod admin@<server_ipv4>:/tmp/.env.prod
ssh -i ~/mykomora-keys/admin_key admin@<server_ipv4> \
  'sudo mv /tmp/.env.prod /opt/mykomora/deploy/.env.prod && sudo chown deploy:deploy /opt/mykomora/deploy/.env.prod && sudo chmod 600 /opt/mykomora/deploy/.env.prod'
```

Then delete your local copy if you don't want a second copy lying around
(your password manager already has the values).

---

## Part 10. Connect GitHub to the server

### 10.1 Get the deploy changes onto `main`
The deploy robot is a file in the repository (`.github/workflows/deploy.yml`).
It only works once it is on the `main` branch: open a pull request with the
deploy files and merge it.

### 10.2 Add the secrets
GitHub → your repository → **Settings → Secrets and variables → Actions**.

**Secrets** (New repository secret):
- `DEPLOY_HOST` → `<server_ipv4>`
- `DEPLOY_SSH_KEY` → the **entire contents** of `~/mykomora-keys/deploy_key`
  (the private one, including the `-----BEGIN` and `-----END` lines)
- `DEPLOY_KNOWN_HOSTS` → run `ssh-keyscan <server_ipv4>` in your terminal and
  paste its whole output. This lets the robot be sure it is talking to *your*
  server.

**Variables** tab → New repository variable:
- `DEPLOY_DOMAIN` → YOUR-ADDRESS

### 10.3 Create the environment
**Settings → Environments → New environment** → name it exactly `production`.
(Optional: tick "Required reviewers" and add yourself, so every deploy waits for
your click.)

---

## Part 11. The first deploy

1. GitHub → **Actions → Deploy → Run workflow** (on `main`) → Run.
2. The first run builds the app (about 5–10 minutes) and publishes three
   packages. The last job, "deploy / production", will most likely **fail**
   this time. That is expected: GitHub creates new packages as private, and the
   server cannot download a private one.
3. Make the packages public: on your GitHub profile or organisation page open
   **Packages**. For each of `mykomora-core-api`, `mykomora-migrate` and
   `mykomora-web`: open it → **Package settings → Change visibility → Public**.
   (They contain only the app's code, no secrets; secrets are in
   `.env.prod` on the server.)
4. Go back to the failed run → **Re-run failed jobs**.

When everything is green, open **https://YOUR-ADDRESS** in a browser. The first
visit can take a few seconds while the server obtains its security certificate.
Sign in with Google (an account you added as a test user), create your family,
and check that adding an item with a photo works.

---

## Part 12. Turn on the nightly backup

The database is backed up every night to the private backup bucket, and old
copies are removed after 14 days (the lifecycle rule you added in Part 6).

```
ssh -i ~/mykomora-keys/admin_key admin@<server_ipv4>
sudo -u deploy crontab -e
```

Add this line at the bottom, save and exit:

```
17 3 * * * /opt/mykomora/deploy/backup.sh >> /home/deploy/backup.log 2>&1
```

Test it once by hand, then look in Cloudflare R2 → `mykomora-backups` for a
file under `postgres/`:

```
sudo -u deploy /opt/mykomora/deploy/backup.sh
```

**A backup you have never restored is a hope, not a backup.** Do the "restore
rehearsal" in `docs/runbook.md` once, soon.

---

## Part 13. From now on: shipping changes

You do nothing special. Merge a pull request into `main`, and:
1. the usual checks run;
2. if they pass, the Deploy robot builds and ships the new version;
3. for a few seconds visitors may wait slightly longer for a page while the
   app restarts; they should not see errors;
4. if the new version does not start properly, the robot puts the previous one
   back and marks the run failed.

**To go back to an older version:** GitHub → Actions → Deploy → Run workflow,
and enter the commit ID of the version you want.

**One rule for database changes:** a change to the database structure must
still work with the *previous* version of the app (add things first, remove
them in a later release). Otherwise going back could break. Ask the person
writing the code to keep this in mind.

---

## If something goes wrong

| Symptom | What to try |
|---|---|
| `tofu init` fails with access denied | Re-check the `AWS_...` variables are from the **state** key (5.2), and `backend.hcl` has the right account ID. |
| `tofu plan` complains about server type | Pick a currently available cheap 2 GB plan and set `server_type`. |
| `tofu init` fails with "tls: handshake failure" talking to `r2.cloudflarestorage.com` | A VPN or your network is interfering. Turn the VPN off (or try your phone's hotspot) and retry. |
| Can't `ssh` to the server | Wait 3 minutes after `tofu apply`. Use `-i ~/mykomora-keys/admin_key`. |
| Browser shows a certificate warning | Your address doesn't point at the server yet (`dig +short YOUR-ADDRESS`), or you left the Cloudflare cloud orange. |
| Google says "redirect_uri_mismatch" | The redirect in Part 8 must match exactly, including `https://` and the path. |
| Google says "access blocked" | The person's email is not in the Test users list. |
| Photos won't upload | The storage rule for your address is missing: re-run `tofu apply` after setting `domain` correctly. |
| Deploy run fails at "Roll out" | Open the run log. The message at the end of the `deploy.sh` output says whether migrations or the app failed. The old version keeps running unless the log says otherwise. |
| Deploy run fails pulling images | The three packages are not public yet (Part 11, step 3). |

Looking at the server directly:

```
ssh -i ~/mykomora-keys/admin_key admin@<server_ipv4>
cd /opt/mykomora
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml ps
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml logs --tail 50 core-api
```

---

## Checklist: everything you need before you start

- [ ] Hetzner account + **HETZNER TOKEN**
- [ ] Cloudflare account with R2 enabled, **ACCOUNT ID**
- [ ] Bucket `mykomora-tfstate`, **STATE KEY ID / SECRET**
- [ ] Buckets `mykomora-media` and `mykomora-backups` with CORS and 14-day rule, **APP KEY ID / SECRET**
- [ ] **STATE PASSPHRASE** (saved!)
- [ ] An address: DuckDNS name or a bought domain
- [ ] SSH keys: `admin_key`, `deploy_key` (+ `.pub`)
- [ ] OpenTofu installed
- [ ] Google OAuth client ID and secret, test users added
- [ ] GitHub secrets: `DEPLOY_HOST`, `DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS`;
      variable `DEPLOY_DOMAIN`; environment `production`
- [ ] `deploy/.env.prod` filled in and copied to the server
- [ ] Deploy files merged to `main`, packages made public
- [ ] Backup cron added and tested, restore rehearsal done
