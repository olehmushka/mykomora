# infra/: OpenTofu for the production server and storage

Creates, in one `tofu apply`:

- Hetzner: SSH key, a **primary IPv4 that survives server rebuilds**, a firewall
  (22, 80, 443/tcp, 443/udp, ping) and the server. First boot installs Docker,
  creates the `admin` (you) and `deploy` (CI) users, disables password SSH and
  adds fail2ban.
- Cloudflare R2: the media and backup buckets, CORS on the media bucket, a
  14-day expiry rule on the backup bucket, and an S3 credential scoped to both.

It does **not** create DNS (DuckDNS has no provider; the IP never changes, so
set the record once) and does **not** put secrets on the server.

## One-time prerequisites (by hand, because they hold the state itself)
1. Cloudflare: create an R2 bucket `mykomora-tfstate` and an API token with
   Object Read & Write on that bucket only. Note the account ID.
2. Cloudflare: create a second API token for tofu with `Workers R2 Storage: Edit`
   and `API Tokens: Edit` on the account.
3. Hetzner: a project and a read & write API token.
4. Two SSH key pairs: yours, and one for CI (`ssh-keygen -t ed25519 -f deploy_key -N ''`).

## Use
```
cd infra
cp backend.hcl.example backend.hcl            # fill in bucket + endpoint
cp terraform.tfvars.example terraform.tfvars  # fill in the non-secrets
export AWS_ACCESS_KEY_ID=...  AWS_SECRET_ACCESS_KEY=...   # state bucket token
export TF_VAR_hcloud_token=...  TF_VAR_cloudflare_api_token=...
export TF_VAR_state_passphrase='long random string, keep it in a password manager'

tofu init -backend-config=backend.hcl
tofu plan
tofu apply
tofu output server_ipv4          # -> DuckDNS record
tofu output s3_endpoint s3_access_key_id
tofu output -raw s3_secret_access_key
```
The state passphrase is not recoverable: lose it and the state is unreadable.

## Things to eyeball on the first `plan`
- `server_type`: Hetzner renames plans; run `hcloud server-type list` and pick
  the cheapest 2 GB shared-vCPU type.
- The R2 permission-group lookup (`r2.tf`) must resolve to exactly one group,
  and that group must allow object read, write and delete (the app's orphan
  sweeper deletes). If the plan shows zero or several, adjust the `name`
  filter. Creating that token by hand and skipping the `cloudflare_api_token`
  resource is a fine fallback.
- The lifecycle `max_age` is in seconds.

`user_data`, `ssh_keys` and `image` are ignored after creation on purpose, so
editing cloud-init never destroys a server that holds the database.
